#!/bin/bash
shopt -s globstar

usage() {
	echo >&2 "Usage: ./update-links.sh [--no-article-dir] DIRECTORY SPACE CONFLUENCE_URL_PREFIX"
}

if [ "$1" = "--help" ]; then
	usage
	exit 0
fi

articleDir=true

while [ $# -ne 0 ]; do
	case "$1" in
		--no-article-dir) articleDir=false
			shift
			;;
		--no-file-size-limit) noFileSizeLimit=true
			shift
			;;
		-- )
			shift
			break
			;;
		* ) break
			;;
	esac
done 

targetDir="$1"
space="$2"
confluenceUrl="$3"

if ! [ -d "$targetDir" ] || [ -z "$space" -o -z "$confluenceUrl" ]; then
	usage
	exit 1
fi

# Make all confluence links easy to find & replace
(cd "$targetDir"

echo "Moving root space pages into their directories..."
for file in **/*.md; do
	matchingDir="${file%.*}"
	if [ -d "$matchingDir" ]; then
		mv "$file" "$matchingDir"
	fi
done
echo "Successfully moved files"

if [ "$articleDir" = true ]; then
	echo "Creating a personal directory for each given article (if it doesn't have one already)"
	for file in **/*.md; do
		baseName="$(basename "$file" .md)"
		dir="$(dirname "$file")"
		directDir="$(basename "$dir")"

		if [ "$directDir" = "$baseName" \
			-a "$(ls "$dir/"*.md | wc -l)" -eq 1 ]; then
			continue
		fi

		mkdir -p "$dir/$baseName"
		mv "$file" "$dir/$baseName/$baseName.md"
	done
	echo "Successfully created directories"
fi


echo "Finding links to be updated..."
changes=""

# Change any confluence links to a uniform format
# Should make further find/replace easier to complete.
changes+="s^\\Q$confluenceUrl\\E([\\^/]*)/pages/([\\^/]*)/[\\^)]*^%%CONFLUENCE_\$1_\$2%%^g; "
# Some links incorrectly convert to local html links of format 'Title_1234567890.html'
changes+="s^\\([\\^(/]*([0-9]{10}).html\\)^(%%CONFLUENCE_${space}_\$1%%)^g; "

# Set confluence space within the front matter
changes+="s/%%CONFLUENCE-SPACE%%/$space/; "
changes+='s/^\\---$/---/; '

for file in **/*.md; do
	vals=$(awk '
	/^\\?---$/ { frontMatter=0 }
	frontMatter && /confluence-id:/ { print "id=" $2 }
	NR == 1 { frontMatter=1 }
	' "$file")

	[ "$vals" ] && declare $vals

	echo "$id => $file"
	changes+="s^%%CONFLUENCE_${space}_${id}%%^</$file>^g; "
done

echo "Applying link changes..."
perl -pi -e "$changes" **/*.md
echo "Link changes applied"

if [ "$articleDir" != true ]; then
	exit 0
fi

echo "Re-organising attachments..."
for file in **/*.md; do
	attachments="$(perl -ne 'for (/\]\((attachments(?:[^)?]|\\\)|\\?)*)(?:\?[^)]*)?\)/g) { print "$_\n"; }' "$file" | sort -u)"
	for attachment in $attachments; do
		dir="$(dirname "$file")"
		base="$(basename "$attachment")"

		# Across dozens of documents, storing files in Git can become a big issue.
		# Thus, we do not migrate attachments over 10MB in size.
		if [ "$noFileSizeLimit" != true \
			-a "$(stat --printf="%s" "$attachment" )" -gt 10000000 ]; then
			echo >&2 "WARNING: Refusing to copy attachment '$attachment' from file '$file'"
			echo >&2 "This file is too big for Git to usually handle. Please upload it to file storage and link from there, or reduce its file-size."
			continue
		fi

		if ! echo "$base" | grep -q '\.'; then
			fileType="$(file -b --extension "$attachment" | sed s^/.*^^)"
			base="$base.$fileType"
		fi

		cp "$attachment" "$dir/$base" # copy in case of duplicates
		perl -pi -e "s^\Q$attachment\E^./$base^g" "$file"
	done
done
echo "Attachments successfully organised"

)
