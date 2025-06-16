#!/bin/bash

# This script helps push your HA VIP Manager code to a new GitHub repository
# Usage: ./push_to_github.sh <github_username> <repository_name>

if [ $# -ne 2 ]; then
    echo "Usage: $0 <github_username> <repository_name>"
    echo "Example: $0 yourusername ha-vip"
    exit 1
fi

USERNAME=$1
REPO_NAME=$2

echo "Setting up GitHub repository for $USERNAME/$REPO_NAME..."

# Set up the remote origin
git remote add origin "https://github.com/$USERNAME/$REPO_NAME.git"

echo "Remote origin added. Before pushing, you need to:"
echo "1. Create a new GitHub repository at: https://github.com/new"
echo "2. Name your repository: $REPO_NAME"
echo "3. Do NOT initialize with README, .gitignore, or license"
echo "4. Click 'Create repository'"

echo ""
read -p "Press Enter after you've created the GitHub repository... "

echo "Pushing main branch to GitHub..."
git push -u origin main

if [ $? -eq 0 ]; then
    echo "Main branch pushed successfully!"
    
    echo "Pushing tags to GitHub..."
    git push --tags
    
    if [ $? -eq 0 ]; then
        echo "Tags pushed successfully! GitHub Actions will automatically build releases from tags."
        echo ""
        echo "Success! Your code is now on GitHub at: https://github.com/$USERNAME/$REPO_NAME"
        echo ""
        echo "Next steps:"
        echo "- Set up GitHub Pages for documentation"
        echo "- Check GitHub Actions workflow status"
        echo "- Configure branch protection rules"
    else
        echo "There was an error pushing tags to GitHub."
    fi
else
    echo "There was an error pushing to GitHub."
    echo "Make sure your GitHub repository exists and you have the correct permissions."
    echo "You might need to configure your GitHub credentials first."
fi
