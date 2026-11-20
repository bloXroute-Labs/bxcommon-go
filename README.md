# bxcommon-go
This is a public library which contains common packages used by multiple projects within bloXroute Labs.

`sdnsdk/message.Account` reads both `secret_hash` and `secret_hash_old` from bxapi.
`ValidateSecret` rejects empty secrets and accepts a match against either hash.