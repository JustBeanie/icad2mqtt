# GitHub Repository Setup Guide

This repository is already connected to `https://github.com/JustBeanie/icad2mqtt`.
For a new checkout, the normal workflow is:

## 1. Create a GitHub Repository (only if you are starting elsewhere)

1. Go to [GitHub](https://github.com) and sign in
2. Click the "+" icon in the top right corner
3. Select "New repository"
4. Name it `icad2mqtt` (or your preferred name)
5. **Do NOT** initialize with a README, .gitignore, or license (we already have these)
6. Click "Create repository"

## 2. Connect Your Local Repository to GitHub

After creating the repository on GitHub, add the remote and push the tracked branch:

```bash
git remote add origin https://github.com/JustBeanie/icad2mqtt.git
git push -u origin master
```

If your repository uses `main` instead, rename the local branch first:
```bash
git branch -M main
git push -u origin main
```

## 3. Enable GitHub Actions

GitHub Actions will automatically run on push and pull requests. The workflow will:
- Run tests
- Build the application
- Build Docker images

## 4. Optional: Add Topics and Description

On your GitHub repository page:
- Add topics like: `go`, `mqtt`, `home-assistant`, `docker`, `911`, `cad`
- Add a description: "ICAD to MQTT Bridge - Fetch 911 CAD events and publish to MQTT"

## 5. Optional: Create a Release

1. Go to the "Releases" section
2. Click "Create a new release"
3. Tag version: `v1.0.0`
4. Release title: `v1.0.0`
5. Add release notes describing the initial release
6. Click "Publish release"

That's it! Your repository is now on GitHub and ready for collaboration.
