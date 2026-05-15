#!/bin/bash

if [[ ! -d "$HOME/.local/share/applications" ]]; then
  mkdir -p "$HOME/.local/share/applications"
fi

cat >"$HOME/.local/share/applications/yarr.desktop" <<END
[Desktop Entry]
Name=yarr
Exec=$HOME/.local/bin/yarr -open
Icon=yarr
Type=Application
Categories=Internet;Network;News;Feed;
END

if [[ ! -d "$HOME/.local/share/icons" ]]; then
  mkdir -p "$HOME/.local/share/icons"
fi

cat >"$HOME/.local/share/icons/yarr.svg" <<END
<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64" role="img" aria-label="yarr">
  <rect width="64" height="64" rx="16" fill="#102620"/>
  <path fill="#fff7e6" d="M18 10h24l8 8v34a6 6 0 0 1-6 6H18a6 6 0 0 1-6-6V16a6 6 0 0 1 6-6Z"/>
  <path fill="#dfeadd" d="M42 10v8h8Z"/>
  <circle cx="23" cy="42" r="4" fill="#f59f2f"/>
  <path d="M23 30c6.6 0 12 5.4 12 12" fill="none" stroke="#f59f2f" stroke-width="5" stroke-linecap="round"/>
  <path d="M23 20c12.2 0 22 9.8 22 22" fill="none" stroke="#f59f2f" stroke-width="5" stroke-linecap="round"/>
</svg>
END
