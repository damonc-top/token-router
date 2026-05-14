#!/bin/bash

launchctl unload ~/Library/LaunchAgents/com.cloudflared.tokenrouter.plist 2>/dev/null
echo "cloudflared tunnel stopped"

launchctl unload ~/Library/LaunchAgents/com.new-api.plist 2>/dev/null
echo "new-api stopped"
