# historical obfuscation check: v0.1.9

the obfuscated release described below received multiple antivirus detections. the standard go profile was evaluated for v0.1.10; v0.1.12 restores garble at the owner's request. the original measurements below describe v0.1.9 only. the cause of the detections requires comparison and vendor analysis.

2026-10-22. the owner limited this work to basic protection and is considering server-side computation. no experimental control-flow transforms, packers or anti-debugging were added.

- the release profile uses garble v0.17.0 with identifier/literal obfuscation, go 1.26.10, stripped debugging data and trimmed paths. the wails rpc surface remains readable by design.
- an earlier comparison found 456 distinct readable internal names in the plain executable and none in the protected executable. the tested engine function name, licensing endpoint and expiry-error string disappeared from direct byte searches.
- rea 6.7.0's go metadata reader reported the protected build's compiler version as `unknown`, with no decoded main-module, dependency or build-setting records. this says nothing about whether machine instructions can be decompiled.
- ghidra 12.1.4 could import the plain pe but its go type analyzer rejected go 1.26.10. the deeper native comparison was stopped; no resistance to ghidra or ai reconstruction is claimed.
- macos detector samples retained zero allocations; protected samples were about 84 ns/tick for two rules and 226 ns/tick for ten rules. these are local microbenchmarks, not windows or end-to-end latency guarantees.
- a backend burst of 1500 alerts took about 30–31 ms in both profiles. literal obfuscation increased allocation counts in that benchmark. the protected macos benchmark needed external linking because this garble release produced a missing-uuid failure with the default linker on this host; the shipped executable is a separate windows build.

protocol/manager/server/controller tests pass under the race detector. frontend validation passes with 121 tests and one existing skipped test; lint has no errors and one warning in the existing alerts table virtualizer. production frontend and protected windows cross-build pass. the final executable's hash and direct byte checks are in `build/bin/protection-audit/basic-check.json`.

windows smbios, dpapi, native frameless rendering, dpi and suspend/resume still require a real windows acceptance run. basic obfuscation does not prevent patching a local executable or reconstructing its behavior; server-side execution is the way to keep selected algorithms out of the distributed binary.

## standard build comparison: 22 october 2026

the same v0.1.9 source was rebuilt with the ordinary go compiler, no garble, packer or string obfuscation, and no debug stripping. windows gui mode and trimmed paths remain. licensing, hardware binding and signature verification were preserved. independent working-tree ui changes were excluded from this controlled comparison.

sha256: `1c3c308eb09e190c1090f1d2257154fbfc4fa3a03e53ebb34c5ff960cf809f78`. [virustotal report](https://www.virustotal.com/gui/file/1c3c308eb09e190c1090f1d2257154fbfc4fa3a03e53ebb34c5ff960cf809f78): 3/69 detections (crowdstrike falcon, elastic, trapmine); maxsecure and tehtris timed out. microsoft, avast/avg, avira, bkav, mcafee, sophos and withsecure reported undetected in this scan. this improves on the owner's supplied list of 12 detections for the obfuscated release, but does not meet the requested zero-detection target or establish safety by itself.

the candidate is `build/bin/Mythra-standard.exe`. the published release and `/download` have not been replaced. the report screenshot and structured results are retained under `build/bin/antivirus-check/`.

## held release candidate: v0.1.10

the windows ci build at `7b088f8` passed checks, and binary-mode govulncheck found no known vulnerabilities. sha256: `c1d689bcad3f88dd6dd7f0a43020a5a86a53886064cb241a2089bbdb0ac5bfcd`. [virustotal report](https://www.virustotal.com/gui/file/c1d689bcad3f88dd6dd7f0a43020a5a86a53886064cb241a2089bbdb0ac5bfcd): 5/71 detections, including microsoft `Trojan:Win32/Wacatac.B!ml` and maxsecure `Trojan.Malware.300983.susgen`, plus crowdstrike, elastic and trapmine.

the owner accepts generic malicious/suspicious labels but requires no trojan classifications. this candidate therefore remains a draft and has not replaced `/download`. the unsigned control build's result did not carry over to the final release file; neither removal of obfuscation nor a clean vulnerability scan proves the absence of malware or guarantees future antivirus verdicts. the owner does not currently have a code-signing certificate.

## release decision: v0.1.12

the owner authorized restoring garble v0.17.0 and publishing all accumulated changes despite the antivirus results. the zero-trojan publication condition is waived for this release. v0.1.10 remains an unpublished comparison draft; v0.1.12 contains the restored protected profile and the pending table/notification fixes. no clean antivirus verdict is claimed for v0.1.12.

the initial v0.1.11 attempt failed a windows activation test. deterministic regression tests reproduced stale polling overwriting activation errors and newer license events. v0.1.12 invalidates those responses and prevents concurrent state polls. the failed tag is retained without publication.
