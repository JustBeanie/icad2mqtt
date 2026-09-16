# icad2mqtt schema v1 contract

The root binary publishes structured output below `MQTT_BASE_TOPIC` (default
`911/cad`). All structured messages use QoS 1. The raw compatibility message
uses `MQTT_TOPIC` (default `911/cad/events`), QoS 1, is not retained, and is
published only when the fetched HTML changes while `PUBLISH_RAW=true`.

## Topics

| Topic | Retained | Payload |
|---|---:|---|
| `<base>/incidents` | yes | schema v1 snapshot, only when canonical incident content changes |
| `<base>/incident` | no | one schema v1 `new`, `updated`, or `closed` event per transition |
| `<base>/availability` | yes | `online` or `offline`; offline is the MQTT LWT |
| `<base>/health` | yes | schema v1 health object |
| `<base>/counts` | yes | category counts when they change (HA discovery enabled) |

When `HA_DISCOVERY=true`, discovery configs are retained at
`homeassistant/sensor/icad2mqtt_<client_id>/<category>_active/config` for
`fire`, `ems`, `police`, `other`, `unknown`, and `total`. They expose counts
only; no incident is an HA entity. When discovery is disabled, the root binary
clears those six retained configs once at startup.

## Schema v1

A snapshot contains `schema`, `source`, `page_updated_at`, `fetched_at`, and an
`incidents` array. Each incident contains `id`, `agency`, `received_at`,
`received_at_raw`, `type`, `address_raw`, `address_clean`, `municipality`,
`cross_streets_raw`, `cross_streets`, and `status`. Agency includes `name`,
`key`, `category`, and `category_source`; type includes `raw`, `key`, and an
optional `code`. The invented example below is intentionally not source data:

```json
{"schema":1,"source":"ongov-911events","page_updated_at":"2026-09-14T10:00:00-04:00","fetched_at":"2026-09-14T14:00:05Z","incidents":[{"id":"0123456789abcdef","agency":{"name":"Example Fire","key":"example fire","category":"fire","category_source":"agency_name"},"received_at":"2026-09-14T09:59:00-04:00","received_at_raw":"09/14/26 09:59","type":{"raw":"FIRE-101 Sample","key":"fire-101-sample"},"address_raw":"10 Invented Way","address_clean":"10 INVENTED WAY","municipality":{"raw":"X01","name":null},"cross_streets_raw":"Fiction Avenue & Sample Road","cross_streets":["Fiction Avenue","Sample Road"],"status":"active"}]}
```

An event is `{"schema":1,"event":"new|updated|closed","incident":...}`.
The first valid snapshot seeds state and emits no `new` events. On restart the
process seeds again; consumers must use the retained snapshot as authoritative
state and events only as transitions. Events are deterministic by incident ID.

Health is `{schema,status,last_success_at,last_error_at,last_error_reason,
consecutive_failures,polls_total,page_errors_total,rows_skipped_total,
rows_dropped_total,duplicates_total,incidents_active,page_updated_at,version}`.
Status is `failing` at three consecutive fetch failures, `degraded` after
skipped/dropped rows or an unparsable page timestamp, and `ok` otherwise.

Counts are `{fire,ems,police,other,unknown,total}`. Discovery value templates
select one count from `<base>/counts`; discovery includes a device block and
measurement state class.

## Field rules

HTML text is whitespace-collapsed and entities are decoded. Agency category is
derived from agency name: fire terms map to `fire`, EMS/medical/ambulance/rescue
squad to `ems`, police/sheriff/state police/troop to `police`, and all remaining
names to `unknown`. IDs are stable hashes of agency, normalized received time,
uppercase clean address, and municipality. Timestamps use New York time and
reject nonexistent DST local times. Cross streets split on ` & `. Short rows
are skipped; invalid times are dropped with a reason counter; duplicate IDs are
counted and removed. A malformed page does not replace the last valid snapshot.

## Privacy and ToneWatch

Addresses and cross streets are present in the retained snapshot. Use broker
ACLs and an appropriate retention policy. Logs contain only bounded reason,
count, and topic information; they never contain incident fields or raw HTML.

ToneWatch should subscribe to `<base>/incident` for transitions and bootstrap
its state from the retained `<base>/incidents` snapshot. It should tolerate a
restart seed with no events and reconcile periodically from the snapshot.

The Home Assistant add-on currently publishes the raw topic only; structured
topics and discovery require running the main icad2mqtt binary (for example the
Docker image). Add-on support is planned.
