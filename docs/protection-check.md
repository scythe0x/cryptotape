# basic protection check

2026-10-22. the owner limited this work to basic protection and is considering server-side computation. no experimental control-flow transforms, packers or anti-debugging were added.

- the release profile uses garble v0.17.0 with identifier/literal obfuscation, go 1.26.10, stripped debugging data and trimmed paths. the wails rpc surface remains readable by design.
- an earlier comparison found 456 distinct readable internal names in the plain executable and none in the protected executable. the tested engine function name, licensing endpoint and expiry-error string disappeared from direct byte searches.
- rea 6.7.0's go metadata reader reported the protected build's compiler version as `unknown`, with no decoded main-module, dependency or build-setting records. this says nothing about whether machine instructions can be decompiled.
- ghidra 12.1.4 could import the plain pe but its go type analyzer rejected go 1.26.10. the deeper native comparison was stopped; no resistance to ghidra or ai reconstruction is claimed.
- macos detector samples retained zero allocations; protected samples were about 84 ns/tick for two rules and 226 ns/tick for ten rules. these are local microbenchmarks, not windows or end-to-end latency guarantees.
- a backend burst of 1500 alerts took about 30–31 ms in both profiles. literal obfuscation increased allocation counts in that benchmark. the protected macos benchmark needed external linking because this garble release produced a missing-uuid failure with the default linker on this host; the shipped executable is a separate windows build.

protocol/manager/server/controller tests pass under the race detector. frontend validation passes with 121 tests and one existing skipped test; lint has no errors and one warning in the existing alerts table virtualizer. production frontend and protected windows cross-build pass. the final executable's hash and direct byte checks are in `build/bin/protection-audit/basic-check.json`.

windows smbios, dpapi, native frameless rendering, dpi and suspend/resume still require a real windows acceptance run. basic obfuscation does not prevent patching a local executable or reconstructing its behavior; server-side execution is the way to keep selected algorithms out of the distributed binary.
