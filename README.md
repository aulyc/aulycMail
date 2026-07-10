![Logo](frontend/src/assets/images/logo-universal.png)

# aulycmail - A Lightweight E-Mail Client

Maintained by: @aulyc

![screenshot](docs/ss.png)

## Summary

aulycmail is a standalone desktop e-mail client focused on:

- Resource efficiency
- Clean desktop UX
- IMAP/SMTP mail support
- Local contacts and autocomplete
- Local-first mail storage and search
- macOS desktop distribution

## OS Support

This checkout targets the macOS desktop build.

## Build

```bash
make build
```

## Release DMG

```bash
make release-dmg \
  SIGN_IDENTITY="Developer ID Application: nan ma (M9M7M2ARFD)" \
  NOTARY_PROFILE=aulyc-notary
```

## Product Links

- Website: https://aulyc.com/aulycmail
- Privacy Policy: https://aulyc.com/aulycmail/privacy
- Terms of Use: https://aulyc.com/aulycmail/terms
