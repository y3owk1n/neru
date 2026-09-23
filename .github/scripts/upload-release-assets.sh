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

# The names are compared in bash and passed to gh as a field, which gh encodes
# into the query string, so no name is spliced into a jq program or a URL.
for file in "$@"; do
    name="$(basename "$file")"
    gh api --paginate "repos/$GITHUB_REPOSITORY/releases/$id/assets" \
        --jq '.[] | "\(.id) \(.name)"' |
        while read -r asset asset_name; do
            if [ "$asset_name" = "$name" ]; then
                gh api -X DELETE "repos/$GITHUB_REPOSITORY/releases/assets/$asset"
            fi
        done
    gh api -X POST "https://uploads.github.com/repos/$GITHUB_REPOSITORY/releases/$id/assets" \
        -f name="$name" -H "Content-Type: application/octet-stream" --input "$file" --silent
done
