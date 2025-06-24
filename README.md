## Alistea

## Config
put config.json in the root directory of your project.
```
{
    "addr": ":80",
    "music": {
        "netease": "...",
        "cookie": "...",
        "qq": "..."
    },
    "qiniu": {
        "ak": "",
        "sk": ""
    },
    "debug": ...,
    "pgsql": "..."
}
```

## Build and run
```bash
go build && ./alisten
```