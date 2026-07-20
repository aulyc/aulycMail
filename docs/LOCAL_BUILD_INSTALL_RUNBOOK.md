# Local Build And Install Runbook

This records the local macOS rebuild/install flow used for this checkout.

## Current Environment

- Repository: `/Users/crp/Projects/aulyc/aulycMail`
- Installed app: `/Applications/aulycMail.app`
- Wails CLI: `/Users/crp/go/bin/wails`
- Node/npm: `/Users/crp/.nvm/versions/node/v24.13.0/bin`
- Build target: `make install-darwin`

## Required Order

1. Rebuild and install with the local Node and Wails paths. The `install-darwin`
   target quits a running installed `aulycMail` process or legacy `aulycmail`
   process before replacing the bundle.

   ```sh
   /bin/zsh -lc "PATH=/Users/crp/.nvm/versions/node/v24.13.0/bin:/Users/crp/go/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin make install-darwin"
   ```

2. If install fails because the app is still running after the built-in wait,
   quit it manually and retry. AppleScript `-128` during the quit request is
   handled by `make install-darwin` as long as the process exits afterward.

   ```sh
   osascript -e 'tell application id "com.aulyc.aulycmail" to quit'
   ```

3. Verify the installed app bundle.

   ```sh
   ls -ld /Applications/aulycMail.app /Applications/aulycMail.app/Contents/MacOS/aulycMail
   codesign --verify --deep --strict --verbose=2 /Applications/aulycMail.app
   ```

## Last Confirmed Run Before Product Rename

- Date/time: 2026-07-01 23:21 CST
- Running app PID before quit: a running `aulycmail` process was detected by `make install-darwin`
- Quit result: the quit target treats AppleScript `-128` ("user cancelled") as non-fatal when the follow-up process-table check shows no installed `aulycmail` main process
- Install result: success
- Installed binary: `/Applications/aulycmail.app/Contents/MacOS/aulycmail`
- Installed timestamp observed: `Jul 1 23:21`
- Codesign verification: valid on disk; satisfies Designated Requirement

This historical record intentionally retains the lowercase App and executable
paths that were verified before the `aulycMail` product rename.

## Notes

- `make install-darwin` runs `quit-running-darwin` first so a running app process does not keep testing the old binary.
- The elevated/sanitized command environment may not include `npm`; include the explicit Node/npm path above.
- `wails build` regenerates `frontend/wailsjs/go/*` bindings. Review those generated diffs before committing.
