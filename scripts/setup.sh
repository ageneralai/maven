#!/usr/bin/env bash
set -euo pipefail

CONFIG_DIR="${HOME}/.maven"
CONFIG_FILE="${CONFIG_DIR}/config.json"

echo "=== Maven setup ==="
echo ""

if [ -f "$CONFIG_FILE" ]; then
    echo "Config already exists: $CONFIG_FILE"
    read -rp "Overwrite? [y/N] " overwrite
    if [[ ! "$overwrite" =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 0
    fi
fi

echo ""
echo "--- Provider ---"
read -rp "Provider type [anthropic/openai] (default: anthropic): " PROVIDER_TYPE
PROVIDER_TYPE="${PROVIDER_TYPE:-anthropic}"
read -rp "API Key: " API_KEY
read -rp "Base URL (leave empty for default): " BASE_URL

echo ""
echo "--- Telegram ---"
read -rp "Enable Telegram? [y/N]: " TG_ENABLED
if [[ "$TG_ENABLED" =~ ^[Yy]$ ]]; then
    TG_ENABLED="true"
    read -rp "Bot Token: " TG_TOKEN
else
    TG_ENABLED="false"
    TG_TOKEN=""
fi

echo ""
echo "--- Feishu ---"
read -rp "Enable Feishu? [y/N]: " FEISHU_ENABLED
if [[ "$FEISHU_ENABLED" =~ ^[Yy]$ ]]; then
    FEISHU_ENABLED="true"
    read -rp "App ID: " FEISHU_APP_ID
    read -rp "App Secret: " FEISHU_APP_SECRET
    read -rp "Verification Token: " FEISHU_VTOKEN
    read -rp "Webhook port (default: 9876): " FEISHU_PORT
    FEISHU_PORT="${FEISHU_PORT:-9876}"
else
    FEISHU_ENABLED="false"
    FEISHU_APP_ID=""
    FEISHU_APP_SECRET=""
    FEISHU_VTOKEN=""
    FEISHU_PORT="9876"
fi

echo ""
echo "--- WeCom ---"
read -rp "Enable WeCom? [y/N]: " WECOM_ENABLED
if [[ "$WECOM_ENABLED" =~ ^[Yy]$ ]]; then
    WECOM_ENABLED="true"
    read -rp "Token: " WECOM_TOKEN
    read -rp "EncodingAESKey (43 chars): " WECOM_AES_KEY
    read -rp "ReceiveID (leave empty to skip strict check): " WECOM_RECEIVE_ID
    read -rp "Callback port (default: 9886): " WECOM_PORT
    WECOM_PORT="${WECOM_PORT:-9886}"
else
    WECOM_ENABLED="false"
    WECOM_TOKEN=""
    WECOM_AES_KEY=""
    WECOM_RECEIVE_ID=""
    WECOM_PORT="9886"
fi

echo ""
echo "--- Web UI ---"
read -rp "Enable Web UI? [y/N]: " WEB_ENABLED
if [[ "$WEB_ENABLED" =~ ^[Yy]$ ]]; then
    WEB_ENABLED="true"
else
    WEB_ENABLED="false"
fi

mkdir -p "$CONFIG_DIR"

cat > "$CONFIG_FILE" <<EOF_JSON
{
  "agent": {
    "workspace": "${HOME}/.maven/workspace",
    "model": "claude-sonnet-4-5-20250929",
    "maxTokens": 8192,
    "temperature": 0.7,
    "maxToolIterations": 20
  },
  "provider": {
    "type": "${PROVIDER_TYPE}",
    "apiKey": "${API_KEY}",
    "baseUrl": "${BASE_URL}"
  },
  "channels": {
    "telegram": {
      "enabled": ${TG_ENABLED},
      "token": "${TG_TOKEN}",
      "allowFrom": []
    },
    "feishu": {
      "enabled": ${FEISHU_ENABLED},
      "appId": "${FEISHU_APP_ID}",
      "appSecret": "${FEISHU_APP_SECRET}",
      "verificationToken": "${FEISHU_VTOKEN}",
      "port": ${FEISHU_PORT},
      "allowFrom": []
    },
    "wecom": {
      "enabled": ${WECOM_ENABLED},
      "token": "${WECOM_TOKEN}",
      "encodingAESKey": "${WECOM_AES_KEY}",
      "receiveId": "${WECOM_RECEIVE_ID}",
      "port": ${WECOM_PORT},
      "allowFrom": []
    },
    "whatsapp": {
      "enabled": false,
      "allowFrom": []
    },
    "matrix": {
      "enabled": false,
      "homeserver": "",
      "accessToken": "",
      "userId": "",
      "allowFrom": [],
      "allowRooms": []
    },
    "web": {
      "enabled": ${WEB_ENABLED},
      "allowFrom": [],
      "voice": {
        "enabled": false
      }
    }
  },
  "tools": {
    "execTimeout": 60,
    "restrictToWorkspace": true,
    "task": {
      "enabled": false
    },
    "acp": {
      "enabled": false,
      "agents": {}
    }
  },
  "skills": {
    "enabled": true,
    "dir": ""
  },
  "autoCompact": {
    "enabled": false,
    "threshold": 0.8,
    "preserveCount": 5
  },
  "gateway": {
    "host": "0.0.0.0",
    "port": 18790,
    "hotReload": false,
    "reloadDebounceMs": 800,
    "cron": {
      "maxConcurrentRuns": 1
    }
  }
}
EOF_JSON

chmod 600 "$CONFIG_FILE"

echo ""
echo "Config written: $CONFIG_FILE"
echo ""
echo "Next:"
echo "  make onboard   # init workspace"
echo "  make gateway   # start"
if [ "$FEISHU_ENABLED" = "true" ]; then
    echo "  make tunnel    # cloudflared tunnel for Feishu webhook"
fi
if [ "$WECOM_ENABLED" = "true" ]; then
    echo "  Configure callback URL to /wecom/bot"
fi
if [ "$WEB_ENABLED" = "true" ]; then
    echo "  Open http://localhost:18790"
fi
