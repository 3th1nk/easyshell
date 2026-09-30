# EasyShell v2

English | [简体中文](README.zh-CN.md)

* Execute local commands (Windows/Linux) and run interactive commands remotely on hosts and network devices over SSH/TELNET
* Unified `Shell` interface abstraction: CmdShell/SshShell/TelnetShell are interchangeable
* Custom prompt matching rules supported; the default rule self-corrects (AutoPrompt). Prompt/interceptor matching only scans a tail window — no linear degradation even on huge config lines (16MB+)
* Auto-detects GB18030 encoding and converts it to UTF-8 by default; custom decoders supported. Only complete lines are decoded, so multi-byte characters are never truncated across network packet boundaries
* Stateful character filter: escape sequences (including string sequences such as OSC/DCS) are safely stripped across packet boundaries, backspace retreats by rune, EL/ED erase applies to the current line in real time, CRLF normalized (including H3C's `\r\r\n`)
* Built-in interceptors: password prompt (Password), option prompt (AlwaysYes/AlwaysNo), automatic paging on network devices (More), continue confirmation (Continue). After a match, the unfinished line is discarded before the response is written — no output sticking
* Deferred output delivery (by time interval or accumulated size); three stderr strategies (Error/Output/Ignore)
* Record raw input/output and replay it (record package, binary frame format with timestamps and direction)
* Device error detection: command parse errors (Cisco IOS/NX-OS/H3C/Huawei/Juniper; Cisco-like syntax such as Ruijie is covered automatically) immediately return *easyshell.DeviceError, with Fail/Collect policies; login banners are excluded from detection
* Lean API parameters: per-call options (prompt override/interceptors/command-level timeout) are unified into the variadic RunOptions struct
* Jump host/bastion: multi-level proxy chain (SshConfig.Proxy); SSH Agent authentication (UseAgent)
* Vendor drivers (VendorProfile): paging disable / save config (incl. confirmation interaction) / error patterns consolidated; custom vendor registration supported
* Keep-alive for long-lived connections (Config.KeepAlive): periodic probe + failure threshold + dead callback, preventing idle disconnects
* Command-level prompt override (RunOptions.Prompt): strict matching for dynamically changing prompts; config-mode state inference (InConfigMode)
* Structured logging hook (Config.Logger *slog.Logger): key session events integrate into ops logging
* Multi-command scripts (RunScript): sequential execution, failures pinpointed to the exact command
* SFTP file upload (temp file + rename atomic write) / download / recursive delete
* Concurrency-safe: Prompt/IsPrompt can be queried concurrently during a read; concurrent Read is rejected with an explicit error
* Offline testable: built-in mock SSH/Telnet/SFTP servers (internal/testsrv)

## Installation

```
go get github.com/3th1nk/easyshell/v2
```

Requires Go 1.21+.

## Packages

Callers only need to care about 4 public packages, each with a single responsibility:

| Package | Responsibility |
|---|---|
| `easyshell` (this package) | Shell interface and all high-level APIs (single entry point) |
| `interceptor` | Output interceptors (password/paging/options/custom) |
| `filter` | Character filter (built-in by default, custom implementations supported) |
| `record` | Recording and replay |

Implementation details such as the read loop and error types are all consolidated under `internal/` and exposed through aliases in this package
(`easyshell.Error`, `easyshell.IsTimeout`, `easyshell.RunOptions`, `easyshell.Config`, etc.).

## Code Snippets

- Run a command over SSH
```go
    s, err := easyshell.NewSshShell(easyshell.SshConfig{
        Credential: easyshell.SshCredential{
            Host: "192.0.2.1", Port: 22, User: "admin", Password: "***",
            Timeout: 5 * time.Second, InsecureAlgorithms: true,
        },
    })
    if err != nil {
        return err
    }
    defer s.Close()

    ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
    defer cancel()

    // Run = write command + read output until the prompt
    err = s.Run(ctx, "display version", func(lines []string) {
        for _, line := range lines {
            fmt.Println(line)
        }
    })
```

- Password interaction (su/sudo)
```go
    err = s.Run(ctx, "su root", nil,
        interceptor.Password("Password:", rootPassword))
    _, err = easyshell.ExitCode(ctx, s) // echo $? to get the exit code of the previous command
```

- Dynamically changing prompts (entering config mode, su/sudo): command-level prompt override + session state inference
```go
    // RunOptions.Prompt specifies the end prompt for this command only (strict match, the default
    //  loose rule is bypassed): avoids both false matches from the loose rule and timeouts caused
    //  by the changed prompt no longer matching
    err = s.Run(ctx, "system-view", onOut,
        easyshell.RunOptions{Prompt: regexp.MustCompile(`\[SW03[\]-][^>]*\]\s*$`)})
    if s.InConfigMode() { // heuristic: H3C/Huawei [name] views, Cisco (config) style
        // continue running commands in config mode...
        err = s.Run(ctx, "quit", nil)
    }
```

- Jump host/bastion (multi-level chain)
```go
    s, err := easyshell.NewSshShell(easyshell.SshConfig{
        Credential: targetCred,
        Proxy: &easyshell.ProxyConfig{ // multi-level chain: a Proxy can itself specify another Proxy
            Credential: bastionCred,
        },
    })
```

- Multi-command script (failure pinpointed to the command)
```go
    err = easyshell.RunScript(ctx, s, func(cmd string, lines []string) {
        fmt.Println("==", cmd)
    }, "screen-length disable", "display version", "display clock")
```

- Command-level timeout (independent of ctx: loosen for slow commands, tighten for interactive ones)
```go
    // Timeout only constrains this call (loosen ping/copy etc. individually without enlarging the
    //  whole session ctx); timeout errors can be checked via easyshell.IsTimeout;
    //  zero means no limit and follows ctx
    err = s.Run(ctx, "ping -c 100 192.0.2.1", nil, easyshell.RunOptions{Timeout: 2 * time.Minute})
```

- Vendor driver and saving configuration
```go
    s.Run(ctx, easyshell.VendorProfileOf(easyshell.VendorH3C).PagingDisable, nil)
    err = easyshell.SaveConfig(ctx, s, easyshell.VendorHuawei, nil) // handles Y/N confirmation automatically
```

- Keep-alive (prevents idle disconnects by firewalls/NAT)
```go
    s, _ := easyshell.NewSshShell(easyshell.SshConfig{
        Credential: cred,
        Config: easyshell.Config{KeepAlive: &easyshell.KeepAliveConfig{
            Interval: 30 * time.Second, // SSH keepalive requests are injected by the library
            OnDead:   func(err error) { /* reconnect/alert */ },
        }},
    })
```

- File transfer with verification and progress
```go
    err = s.Upload(ctx, "local.bin", "/remote/path.bin", easyshell.TransferOptions{
        Force:    true, // overwrite if the target already exists
        Progress: func(transferred, total int64) { fmt.Printf("\r%d/%d", transferred, total) },
    }) // automatic md5sum verification after upload (NoVerify off); protocol is auto-selected,
       // falling back to SCP when SFTP is unavailable
```

- Export a recording as asciinema (playable directly in players)
```go
    record.DumpAsciinema(recFile, os.Stdout) // input frames are excluded to prevent secret leakage
```

- Device error detection (command parse errors in output fail automatically)
```go
    // Enabled by default: a Cisco IOS/NX-OS/H3C/Huawei/Juniper command parse error in the output
    //  returns *easyshell.DeviceError
    //  (login banners are excluded, so devices that print error-style text at login do not cause
    //  false positives; references in docs/ERRDETECT-REFERENCES.md)
    err = s.Run(ctx, "disp lay ver", onOut)
    var de *easyshell.DeviceError
    if errors.As(err, &de) {
        fmt.Println("command error:", de.Cmd, de.Line, de.PatternName)
    }

    // Policy and patterns are configurable (Config):
    cfg.ErrorPolicy = easyshell.ErrorCollect       // don't abort on a match; aggregate and return at the end
    cfg.ErrorPatterns = append(easyshell.DefaultErrorPatterns(),
        easyshell.NewErrorPattern("my-error", `(?i)^my custom error`))
    cfg.ErrorPatterns = []*easyshell.ErrorPattern{} // explicitly disable detection
```

- Disable paging (recommended before fetching long configs; faster than answering More page by page)
```go
    easyshell.DisablePaging(ctx, s, easyshell.VendorH3C) // best effort: falls back to the More
                                                         // interceptor silently when unrecognized
    s.Run(ctx, "display saved-configuration", onOut)
```

- Telnet / local commands
```go
    ts, err := easyshell.NewTelnetShell(easyshell.TelnetConfig{
        Credential: easyshell.TelnetCredential{Host: "192.0.2.1", User: "admin", Password: "***"},
    })

    cs, err := easyshell.NewCmdShell(ctx, easyshell.CmdConfig{Command: "ping www.baidu.com"})
```

- Recording and replay
```go
    // Option 1 (recommended): the Record config — just set the path; metadata is auto-filled and
    // finalized on Close
    s, _ := easyshell.NewSshShell(easyshell.SshConfig{
        Credential: cred,
        Record:     &easyshell.RecordConfig{Path: "session.eshrec", CaptureInput: true},
    })
    // ... run commands ...
    s.Close()

    // Option 2 (advanced): wire a record.Writer into RawOut/RawIn manually for custom capture pipelines
    rec, _ := record.NewFileWriter("session.eshrec",
        record.Meta{Host: host, Protocol: "ssh"}, record.Options{CaptureInput: true})
    _ = easyshell.Config{RawOut: rec, RawIn: rec.Input()}

    // Replay (in order / at speed multiplier), or feed it back into a Shell interactively
    // (see the record package docs)
    player, _ := record.Open("session.eshrec")
    _ = player.Play(ctx, os.Stdout)
```

## Device Testing

Credentials for real devices are injected via environment variables or a `.env` file (git-ignored); the related tests skip automatically when not configured. See the comment at the top of `test_cred_test.go` for the variable format and the full list:

```
EASYSHELL_TEST_CISCO=admin:password@192.0.2.90
EASYSHELL_TEST_H3C=admin:password@192.0.2.4
```

Test layers: `TestMock*` (offline, run by default) / `TestDevice_*` (real devices, credentials required).

## Documentation

- [v1 → v2 Migration Guide](docs/MIGRATION.md)
- [Roadmap](docs/ROADMAP.md)
- [Changelog](CHANGELOG.md)

## Notes

- The `onOut` callback is invoked **synchronously** in the read loop: blocking it delays reading and prompt detection (the device may be waiting for an interceptor response).
  If heavy processing is needed, async it yourself inside the callback (buffer + goroutine); the drop/backlog policy is up to your business logic
- Concurrent Read on the same Shell returns `easyshell.ErrConcurrentRead` (Prompt/IsPrompt/InConfigMode can be queried concurrently during a read)

## Known Limitations

- When a multi-line interceptor matches, prefix lines within the match window are not returned as output (present since v1, kept unchanged)
- EL/ED erase is handled with the conservative "clear the current line" semantics (no column position info); disable via `filter.Options.Erase=false`
- Decoder contract: the multi-byte characters of the target encoding must not contain the 0x0A byte (GBK/GB18030/UTF-8 satisfy this; UTF-16 is not applicable)
- Recording files are plaintext (including password frames) — keep them safe; secret frames are masked by default on Dump
