#!/bin/sh
set -eu

package_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/../cmd/relay" && pwd)
staging_dir=$(mktemp -d)
trap 'rm -rf -- "$staging_dir"' EXIT

cd "$staging_dir"
fyne package -os linux -release -source-dir "$package_dir" "$@"
tar -xJf Relay.tar.xz

# Fyne writes the icon to the per-user icon directory without a theme.
# Keep the desktop entry's icon name and install the PNG in the fallback theme.
sed 's|/share/icons/$(Icon)|/share/icons/hicolor/512x512/apps/$(Icon)|g' \
    relay/Makefile > relay/Makefile.new
icon_paths=$(grep -Fc '/share/icons/hicolor/512x512/apps/$(Icon)' relay/Makefile.new || true)
if [ "$icon_paths" -ne 2 ]; then
    printf '%s\n' 'Could not find the Fyne user install and uninstall icon paths.' >&2
    exit 1
fi
mv relay/Makefile.new relay/Makefile

tar -cJf Relay.tar.xz relay
mv Relay.tar.xz "$package_dir/Relay.tar.xz"
