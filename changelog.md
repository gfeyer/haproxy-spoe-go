# Changelog

# unreleased

- perf: reuse the KV item buffer across resets instead of dropping it, which was
  the bulk of what decoding a frame allocated
- perf: intern message names and argument keys, which repeat on every frame
- perf: keep a message's KV instead of returning it to the pool and taking a new
  one on every reset
- perf: encode frames into a buffer owned by the frame and write once, removing
  the per frame `bytes.Buffer` and payload slices from the ack path
- perf: grow the frame read buffer in rounded steps so a stream of similarly
  sized frames allocates once
- feat: `frame.SetNoCopyStrings` decodes string arguments in place, removing one
  allocation per string argument. Off by default: it limits the lifetime of
  decoded strings to the handler call
- feat: `frame.MaxFrameLen` bounds the accepted frame length
- fix: a five byte packet declaring a zero frame length made the reader try to
  allocate 4GB
- fix: truncated frames could panic instead of returning an error
- fix: encoding an integer of 2^54 or more could panic, the varint scratch buffer
  was 8 bytes where the encoding needs up to 10

# v1.0.7 (2025-08-18)

- worker: wait for in-flight Notify handlers before closing (#26) Simon Taranto* 

# v1.0.6 (2024-03-13)

- bugfix: `false` value has incorrect encoding value - invert flag and value (#22) Brendan Forster*

# v1.0.5 (2023-05-22)

- some refactoring and remove dependencies
- fix encode binary data, add length

# v1.0.4 (2022-08-23)

- Use plain Action array rather than array of pooled pointers (#13)
- Introduce generic logger interface (#12)
- Update to Go 1.19 (#14)

# v1.0.3 (2021-12-16)

- support for parsing IPv6 addresses (#11)

# v1.0.2 (2021-06-01)

- fix-oom-on-http-requests (#9)

# v1.0.1 (2021-01-07)

- add license
- bugfixes
- fix random error under load 
- move buffer out of loop to reduce mem usage
- by default frame don't have actions and doesn't have ownership on it

# v1.0.0

- initial version
    - fix bug with encode/decode binary data
