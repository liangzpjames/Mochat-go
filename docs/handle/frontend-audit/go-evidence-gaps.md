# Go contract evidence gaps

These legacy contracts have no matching Go registration, handler, store path, or test containing the full mounted method and route. Their `go_evidence` value is `-`; migration work must treat them as blocked until a Go contract is implemented and verified.

- `GET /operation/officialAccount/openUserInfo`
- `GET /sidebar/roomCalendar/index`
- `GET /sidebar/roomCalendar/setRoom`
- `GET /sidebar/roomQuality/deleteRoom`
- `GET /sidebar/roomQuality/index`
- `GET /sidebar/roomQuality/setRoom`
- `GET /sidebar/roomSop/tipSopAddRoom`
- `GET /sidebar/roomSop/tipSopDelRoom`
- `GET /sidebar/roomSop/tipSopList`
- `PUT /sidebar/roomSop/setRoom`
