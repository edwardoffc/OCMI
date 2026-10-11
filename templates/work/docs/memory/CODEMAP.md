# Code entry-point map

Template, not a completed index. The orchestrator fills it only from verified source code.

| Area / task | File and symbol / entry point | Related callers, contracts, and checks | Verification source | Freshness |
| --- | --- | --- | --- | --- |

Verification source: date + commit/snapshot or fingerprints of area files + link to a check/source.
Freshness: verified for the named snapshot, needs-check after an affecting change, unknown for a gap.
One map-wide timestamp does not verify every row. Start with areas of current tasks.
Rename, interface, schema/config, caller, or test/check changes require checking related rows, not the entire map.

The worker receives relevant rows, verifies paths/symbols, searches with rg, and reads current code.
For gaps, expand retrieval within authorized scope: include callers, contracts, tests, and fixtures.
Incomplete search/unavailable input does not establish that a symbol is absent.
Do not limit completeness by file/pass count.
Put correction proposals and evidence in Result; the orchestrator updates the shared map.
Line numbers are optional; the map locates sources and does not prove behavior.
