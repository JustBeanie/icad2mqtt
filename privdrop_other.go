//go:build !linux

package main

func dropPrivileges() error { return nil }
