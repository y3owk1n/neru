#!/usr/bin/env bash
# upload-release-assets.sh TAG FILE...
#
# Uploads each FILE to the release TAG, in place of a file of the same name.
#
# `gh release upload --clobber` finds the file to replace in the list that the
# release object embeds, and GitHub can serve a stale copy of that list, with
# files it already deleted. The delete then fails with a 404. The
# /releases/{id}/assets endpoint lists the files that exist, so this script
# replaces each file by that list.
set -euo pipefail

tag="$1"
shift

id="$(gh api "repos/$GITHUB_REPOSITORY/releases/tags/$tag" --jq .id)"

for file in "$@"; do
    name="$(basename "$file")"
    gh api --paginate "repos/$GITHUB_REPOSITORY/releases/$id/assets" \
        --jq ".[] | select(.name == \"$name\") | .id" |
        while read -r asset; do
            gh api -X DELETE "repos/$GITHUB_REPOSITORY/releases/assets/$asset"
        done
    gh api -X POST "https://uploads.github.com/repos/$GITHUB_REPOSITORY/releases/$id/assets?name=$name" \
        -H "Content-Type: application/octet-stream" --input "$file" --silent
done
