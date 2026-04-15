Using Redis to store user configuration data structure

#### 1. Configuration
##### 1. Global configuration hget structure
xiaozhi:global:config

##### 2. User configuration can override values in configuration file, hget structure
```
xiaozhi:userconfig:{deviceid}
    "llm": {
        "provider": "deepseek",         //Corresponds to key in config file llm
    },
    "tts": {
        "provider": "cosyvoice",        //Corresponds to key in config file tts
    }
```

#### 2. Prompt
##### 1. System prompt get/set
xiaozhi:llm:system:{deviceid}

##### 2. Chat session prompt record sorted set structure
xiaozhi:llm:{deviceid}
