# Changelog

## 1.01
- Fixed a bug that impacted fuzzing performance quite noticeably. The `Repair` function used by the server and client binaries, included trailing zeros of the underlying writeback array (up to 50% of the file size). This did not affect functionality but caused many no-op mutations and odd splicing results.

## 1.00
- Initial public release
