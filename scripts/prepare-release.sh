#!/bin/bash

# Release preparation script
# Automates CHANGELOG updates, version bumping, commit and tag creation

set -e

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Get current version
get_current_version() {
    if git describe --tags --abbrev=0 2>/dev/null; then
        git describe --tags --abbrev=0 | sed 's/^v//'
    else
        echo "0.0.0"
    fi
}

# Pre-release checks
pre_release_checks() {
    echo -e "${BLUE}Running pre-release checks...${NC}"
    
    # Git status check
    if [[ -n $(git status -s) ]]; then
        echo -e "${RED}✗ Working directory is not clean${NC}"
        echo "Please commit or stash your changes before releasing."
        exit 1
    fi
    echo -e "${GREEN}✓ Working directory is clean${NC}"
    
    # Branch check
    current_branch=$(git branch --show-current)
    if [[ "$current_branch" != "main" ]] && [[ "$current_branch" != "master" ]]; then
        echo -e "${YELLOW}⚠ You are not on main/master branch (current: $current_branch)${NC}"
        read -p "Continue anyway? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    else
        echo -e "${GREEN}✓ On main branch${NC}"
    fi
    
    # Remote sync check
    git fetch origin
    LOCAL=$(git rev-parse @)
    REMOTE=$(git rev-parse @{u})
    if [[ "$LOCAL" != "$REMOTE" ]]; then
        echo -e "${RED}✗ Local branch is not in sync with remote${NC}"
        echo "Please pull or push your changes first."
        exit 1
    fi
    echo -e "${GREEN}✓ In sync with remote${NC}"
    
    # Run tests
    echo -e "${BLUE}Running tests...${NC}"
    if make test > /dev/null 2>&1; then
        echo -e "${GREEN}✓ All tests passed${NC}"
    else
        echo -e "${RED}✗ Tests failed${NC}"
        echo "Please fix failing tests before releasing."
        exit 1
    fi
    
    # Check if build works
    echo -e "${BLUE}Testing build...${NC}"
    if make build > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Build successful${NC}"
    else
        echo -e "${RED}✗ Build failed${NC}"
        echo "Please fix build errors before releasing."
        exit 1
    fi
}

# Generate release notes
generate_release_notes() {
    local version=$1
    local previous_version=$2
    local template_file="$PROJECT_ROOT/.github/RELEASE_TEMPLATE.md"
    local output_file="$PROJECT_ROOT/.github/releases/v${version}.md"
    
    mkdir -p "$PROJECT_ROOT/.github/releases"
    
    # Copy template
    cp "$template_file" "$output_file"
    
    # Replace version numbers
    sed -i.bak "s/{{VERSION}}/$version/g" "$output_file"
    sed -i.bak "s/{{PREVIOUS_VERSION}}/$previous_version/g" "$output_file"
    rm "${output_file}.bak"
    
    # Extract changes from CHANGELOG
    if [[ -f "$PROJECT_ROOT/CHANGELOG.md" ]]; then
        # Extract the latest version section from CHANGELOG
        awk "/## \[$version\]/{flag=1; next} /## \[/{flag=0} flag" "$PROJECT_ROOT/CHANGELOG.md" > /tmp/changelog_section.md
        
        # TODO: Insert changelog content into release notes
        # This would require more sophisticated parsing
    fi
    
    echo -e "${GREEN}✓ Release notes generated: $output_file${NC}"
}

# Generate checksums
generate_checksums() {
    local version=$1
    local dist_dir="$PROJECT_ROOT/dist"
    local checksum_file="$dist_dir/checksums.txt"
    
    if [[ ! -d "$dist_dir" ]]; then
        echo -e "${YELLOW}⚠ No dist directory found, skipping checksum generation${NC}"
        return
    fi
    
    echo -e "${BLUE}Generating checksums...${NC}"
    
    cd "$dist_dir"
    sha256sum *.tar.gz *.zip 2>/dev/null > "$checksum_file" || true
    
    if [[ -s "$checksum_file" ]]; then
        echo -e "${GREEN}✓ Checksums generated${NC}"
        cat "$checksum_file"
    else
        echo -e "${YELLOW}⚠ No artifacts found for checksum generation${NC}"
    fi
    
    cd - > /dev/null
}

# Main process
main() {
    echo -e "${BLUE}Preparing release for envy${NC}"
    echo
    
    # Pre-release checks
    pre_release_checks
    echo
    
    # Get version information
    current_version=$(get_current_version)
    echo -e "${BLUE}Current version: v$current_version${NC}"
    
    # Ask for version bump type
    echo "How would you like to bump the version?"
    echo "  1) Patch (bug fixes)"
    echo "  2) Minor (new features)"
    echo "  3) Major (breaking changes)"
    echo "  4) Custom version"
    read -p "Select option (1-4): " version_choice
    
    case $version_choice in
        1) bump_type="patch" ;;
        2) bump_type="minor" ;;
        3) bump_type="major" ;;
        4)
            read -p "Enter custom version (without 'v' prefix): " new_version
            bump_type="custom"
            ;;
        *)
            echo -e "${RED}Invalid option${NC}"
            exit 1
            ;;
    esac
    
    # Bump version
    if [[ "$bump_type" != "custom" ]]; then
        echo -e "${BLUE}Bumping version ($bump_type)...${NC}"
        cd "$PROJECT_ROOT"
        ./scripts/version.sh bump "$bump_type"
        new_version=$(get_current_version)
    else
        # For custom version, we need to manually update
        cd "$PROJECT_ROOT"
        ./scripts/version.sh embed "$new_version"
        
        # Update CHANGELOG
        ./scripts/version.sh bump patch  # This will update CHANGELOG
        
        # Create commit and tag manually
        git add -A
        git commit -m "chore: bump version to v$new_version"
        git tag -a "v$new_version" -m "Release v$new_version"
    fi
    
    echo
    echo -e "${GREEN}✅ Release v$new_version prepared successfully!${NC}"
    echo
    
    # Generate release notes
    generate_release_notes "$new_version" "$current_version"
    
    # Build release artifacts
    echo -e "${BLUE}Building release artifacts...${NC}"
    if make release; then
        echo -e "${GREEN}✓ Release artifacts built${NC}"
        generate_checksums "$new_version"
    else
        echo -e "${YELLOW}⚠ Failed to build release artifacts${NC}"
    fi
    
    echo
    echo -e "${YELLOW}Next steps:${NC}"
    echo "1. Review the changes:"
    echo "   git show"
    echo "2. Push the changes and tag:"
    echo "   git push origin main"
    echo "   git push origin v$new_version"
    echo "3. Create a GitHub release:"
    echo "   - Go to https://github.com/drapon/envy/releases/new"
    echo "   - Select tag: v$new_version"
    echo "   - Use the generated release notes from: .github/releases/v${new_version}.md"
    echo "   - Upload the artifacts from: dist/"
    echo "4. Update Homebrew formula (if applicable)"
    echo
    echo -e "${GREEN}Good luck with your release!${NC}"
}

# Entry point
cd "$PROJECT_ROOT"
main "$@"