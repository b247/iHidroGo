# iHidroGo
This is a Go API client for the Hidroelectrica Romania (iHidro) SEW API server.

## License
Copyright (C) 2026 @b247_eu, https://b247.eu.org

This program is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License along with this program. If not, see <https://opensource.org/license/gpl-3.0/>.

## What it does
* **`-submitIndex <value>`** SubmitSelfMeterRead, submit the self meter index read (`<value>`) to Hidroelectrica
* **`-getIndexHistory`** Fetch index reading history from Hidroelectrica
* **`-getSelfMeterReadAllowed`** Fetch allowed windows dates for index submit.

## Usage
The project is platform-agnostic and produces a single standalone binary, whether downloaded from GitHub Releases or compiled locally using `go build`.

The binary can be used as-is as long as valid authentication data is provided either in `auth.json` (placed alongside the binary) or passed inline via `-authConfig` `'{"user":"your_email@example.com","pass":"your_password","uan":"your_10_digit_hidroelectrica_cod_cont_contract_number"}'`.

**Related projects:**
[iHidroHA](https://github.com/b247/iHidroHA) — Home Assistant HACS integration