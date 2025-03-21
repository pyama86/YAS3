## slackのマニフェスト

下記を設定してください。

```
{
    "display_information": {
        "name": "sssbot",
        "description": "インデント対応を支援します。",
        "background_color": "#697596"
    },
    "features": {
        "bot_user": {
            "display_name": "sssbot",
            "always_online": false
        }
    },
    "oauth_config": {
        "scopes": {
            "bot": [
                "app_mentions:read",
                "channels:manage",
                "channels:read",
                "channels:write.topic",
                "chat:write",
                "groups:read",
                "groups:write.topic",
                "im:read",
                "im:write.topic",
                "mpim:read",
                "mpim:write.topic",
                "pins:read",
                "usergroups:read",
                "users:read",
                "files:write"
            ]
        }
    },
    "settings": {
        "event_subscriptions": {
            "bot_events": [
                "app_mention",
                "channel_archive"
            ]
        },
        "interactivity": {
            "is_enabled": true
        },
        "org_deploy_enabled": false,
        "socket_mode_enabled": true,
        "token_rotation_enabled": false
    }
}
```
