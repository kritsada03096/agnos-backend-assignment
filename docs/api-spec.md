# API specification

Local base URL: `http://localhost:8080`. Bodies require `Content-Type: application/json`. Application responses use `{"success":true,"data":...}` or `{"success":false,"message":"..."}`. Health success is `{"success":true}`. Responses set `Cache-Control: no-store`.

## POST /staff/create

Requires `X-Provisioning-Key: <STAFF_PROVISIONING_KEY>` (administrative design addition).

```json
{"username":"staff01","password":"password123","hospital":"hospital-a"}
```

All fields required; unknown fields and trailing JSON rejected. Username/hospital: 1-64 ASCII characters, first alphanumeric, others alphanumeric or `_`, `.`, `-`; case-sensitive. Password: 8-72 bytes, not trimmed, bcrypt stored. Hospital must exist; username unique per hospital. Body limit 8 KiB.

201:

```json
{"success":true,"data":{"id":1,"hospital_id":1,"username":"staff01"}}
```

Errors: 400 invalid input; 403 provisioning key; 404 hospital missing; 409 duplicate; 415 content type; 500 internal error. Nginx may return 413 (body size) or 429 (rate limit).

## POST /staff/login

Same request fields. Login accepts password lengths 1-72 bytes; newly created accounts require at least eight. Invalid username, password or hospital combinations return the same 401.

200:

```json
{"success":true,"data":{"access_token":"<signed-jwt>","token_type":"Bearer"}}
```

Claims: `staff_id`, `hospital_id`, `username`, `sub` (staff ID string), `iss=agnos`, `aud=[agnos-api]`, `iat`, `exp`. Default lifetime one hour, configurable with `JWT_TTL`. HS256 only. No refresh/logout endpoint.

Errors: 400 input; 401 credentials; 415 content type; 500 internal; Nginx may return 429.

## GET /patient/search

Requires `Authorization: Bearer <token>`. Current staff must still match the token. `hospital`/`hospital_id` query parameters are rejected; scope comes exclusively from authentication.

| Optional parameter | Matching / validation |
| --- | --- |
| `national_id` | Exact string; 1-64 ASCII alphanumerics/hyphens |
| `passport_id` | Exact string; same format |
| `first_name` | Case-insensitive literal substring of Thai OR English first name |
| `middle_name` | Same for middle name |
| `last_name` | Same for last name |
| `date_of_birth` | Valid `YYYY-MM-DD`, exact |
| `phone_number` | Exact string, no normalization |
| `email` | Valid address, exact/case-sensitive |
| `page` | 1-1,000,000; default 1 |
| `limit` | 1-100; default 20 |

Filters are trimmed, limited to 254 bytes and combined with AND. Empty filter values mean omitted. Unknown/repeated parameters or malformed URL encoding return 400. `%`/`_` in names are literal. Results sort by patient ID ascending. No filters lists only this hospital's local records. Pages beyond available data return an empty array. Total counts are not computed.

```http
GET /patient/search?national_id=1111111111111&first_name=Demo
Authorization: Bearer <token>
```

200 example:

```json
{
  "success": true,
  "data": [{
    "id": 1, "hospital_id": 1, "patient_hn": "HN-a-001",
    "national_id": "1111111111111", "passport_id": "DEMO-PASSPORT",
    "first_name_th": "ผู้ป่วยทดสอบ", "middle_name_th": null, "last_name_th": "โรงพยาบาล a",
    "first_name_en": "Demo", "middle_name_en": null, "last_name_en": "Hospital a",
    "date_of_birth": "1990-01-01", "phone_number": "0800000000",
    "email": "demo-a@example.test", "gender": "M"
  }],
  "pagination": {"page": 1, "limit": 20}
}
```

No match: `{"success":true,"data":[],"pagination":{"page":1,"limit":20}}`.

### HIS assumptions

Supplied route: `GET https://hospital-a.api.co.th/patient/search/{id}`. Store base URL without `/patient/search`; client appends the escaped path. Configuration is server-side; redirects are rejected.

Fallback requires an identifier absent locally. With both IDs, request by national ID and validate both returned values. Identifier mismatch is 502 and is not saved. Reapply all original filters after import. An existing identity that fails another filter does not trigger refresh.

Unspecified provider details are implemented as assumptions: 200 contains a direct patient object, 404 means absent, HN is nonempty and stable within a hospital, optional fields may be null, dates use `YYYY-MM-DD`, gender is `M`/`F`/null, and no upstream authentication is required. Maximum response size 1 MiB. Hospital B demo uses this same contract solely as a fixture. Actual provider authentication/contract must be confirmed before live integration.

| Status | Meaning |
| --- | --- |
| 200 | Search completed, including zero matches |
| 400 | Invalid query |
| 401 | Missing/invalid/expired token or stale staff identity |
| 500 | Database/internal failure |
| 502 | HIS transport/status/payload/identifier error or missing configuration |
| 504 | HIS timeout |

## Health and routing

`GET /health/live` returns 200 while serving. `GET /health/ready` returns 200 for a successful DB ping, 503 otherwise; it does not call HIS. Unknown routes return 404 and unsupported methods on known routes return 405. Nginx-generated errors may use its own body format.
