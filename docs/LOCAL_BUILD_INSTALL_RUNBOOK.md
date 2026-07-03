# Local Build And Install Runbook

This records the local macOS rebuild/install flow used for this checkout.

## Current Environment

- Repository: `/Users/crp/Projects/aulycmail`
- Installed app: `/Applications/aulycmail.app`
- Wails CLI: `/Users/crp/go/bin/wails`
- Node/npm: `/Users/crp/.nvm/versions/node/v24.13.0/bin`
- Build target: `make install-darwin`

## Required Order

1. Rebuild and install with the local Node and Wails paths. The `install-darwin`
   target now quits a running installed `aulycmail` process before replacing the
   bundle.

   ```sh
   /bin/zsh -lc "PATH=/Users/crp/.nvm/versions/node/v24.13.0/bin:/Users/crp/go/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin make install-darwin"
   ```

2. If install fails because the app is still running, quit it manually and retry.

   ```sh
   osascript -e 'tell application id "com.aulyc.aulycmail" to quit'
   ```

3. Verify the installed app bundle.

   ```sh
   ls -ld /Applications/aulycmail.app /Applications/aulycmail.app/Contents/MacOS/aulycmail
   codesign --verify --deep --strict --verbose=2 /Applications/aulycmail.app
   ```

## Last Confirmed Run

- Date/time: 2026-07-01 23:21 CST
- Running app PID before quit: a running `aulycmail` process was detected by `make install-darwin`
- Quit result: the first AppleScript quit request returned `-128` ("user cancelled"), but a follow-up process-table check showed no installed `aulycmail` main process; retrying `make install-darwin` then reported no running process and continued
- Install result: success
- Installed binary: `/Applications/aulycmail.app/Contents/MacOS/aulycmail`
- Installed timestamp observed: `Jul 1 23:21`
- Codesign verification: valid on disk; satisfies Designated Requirement

## Notes

- `make install-darwin` runs `quit-running-darwin` first so a running app process does not keep testing the old binary.
- The elevated/sanitized command environment may not include `npm`; include the explicit Node/npm path above.
- `wails build` regenerates `frontend/wailsjs/go/*` bindings. Review those generated diffs before committing.
