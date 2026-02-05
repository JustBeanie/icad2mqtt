# GitHub Repository Setup Guide

Your repository is now ready to be pushed to GitHub! Follow these steps:

## 1. Create a GitHub Repository

1. Go to [GitHub](https://github.com) and sign in
2. Click the "+" icon in the top right corner
3. Select "New repository"
4. Name it `icad2mqtt` (or your preferred name)
5. **Do NOT** initialize with a README, .gitignore, or license (we already have these)
6. Click "Create repository"

## 2. Connect Your Local Repository to GitHub

After creating the repository on GitHub, you'll see instructions. Run these commands (replace `YOUR_USERNAME` with your GitHub username):

```bash
git remote add origin https://github.com/YOUR_USERNAME/icad2mqtt.git
git branch -M main
git push -u origin main
```

If your default branch is `master` instead of `main`, use:
```bash
git remote add origin https://github.com/YOUR_USERNAME/icad2mqtt.git
git push -u origin master
```

## 3. Update Repository URLs

After pushing, update these files with your actual GitHub username:

1. **README.md**: Replace `YOUR_USERNAME` with your GitHub username in the clone URL
2. **addon/addon.json**: Replace `YOUR_USERNAME` in the `url` and `image` fields

## 4. Enable GitHub Actions

GitHub Actions will automatically run on push and pull requests. The workflow will:
- Run tests
- Build the application
- Build Docker images

## 5. Optional: Add Topics and Description

On your GitHub repository page:
- Add topics like: `go`, `mqtt`, `home-assistant`, `docker`, `911`, `cad`
- Add a description: "ICAD to MQTT Bridge - Fetch 911 CAD events and publish to MQTT"

## 6. Optional: Create a Release

1. Go to the "Releases" section
2. Click "Create a new release"
3. Tag version: `v1.0.0`
4. Release title: `v1.0.0`
5. Add release notes describing the initial release
6. Click "Publish release"

That's it! Your repository is now on GitHub and ready for collaboration.