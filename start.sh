#!/bin/bash

# 启动 new-api
launchctl load ~/Library/LaunchAgents/com.new-api.plist 2>/dev/null
echo "new-api started"

# 等待 new-api 就绪
sleep 3

# 启动 cloudflared 隧道
launchctl load ~/Library/LaunchAgents/com.cloudflared.tokenrouter.plist 2>/dev/null
echo "cloudflared tunnel started"

echo "All services running in background"
echo "Logs: tail -f /Users/mac/github/new-api/new-api.log"
echo "      tail -f /Users/mac/github/new-api/cloudflared.log"
