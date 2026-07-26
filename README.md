# iHidroGo
This is a Go API client for the Hidroelectrica Romania (iHidro) SEW API server.

**What it does:**
```
-submitIndex <value> (aka SubmitSelfMeterRead)
-getIndexHistory (aka GetMeterReadHistory )
```
And behind the scenes, required by the main exposed methods:
```
-getSelfMeterReadAllowed (aka GetWindowDates)

```

The project is platform-agnostic. Releases can be used as-is as long as valid authentication data is provided either in `auth.json` (placed alongside the binary) or passed inline via `-authConfig` `'{"user":"your_email@example.com","pass":"your_password","uan":"your_10_digit_hidroelectrica_cod_cont_contract_number"}'`.

**Related projects**
***
[iHidroHA](https://github.com/b247/iHidroHA) — Home Assistant HACS integration