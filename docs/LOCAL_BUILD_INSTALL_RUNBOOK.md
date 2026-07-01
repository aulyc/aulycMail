# Local Build And Install Runbook

This records the local macOS rebuild/install flow used for this checkout.

## Current Environment

- Repository: `/Users/crp/Projects/aulycmail`
- Installed app: `/Applications/aulycmail.app`
- Wails CLI: `/Users/crp/go/bin/wails`
- Node/npm: `/Users/crp/.nvm/versions/node/v24.13.0/bin`
- Build target: `make install-darwin`

## Required Order

1. Quit the running installed app first.

   ```sh
   osascript -e 'tell application id "com.aulyc.aulycmail" to quit'
   ```

2. Verify the old process is gone.

   ```sh
   ps -p <old-pid> -o pid=,command=
   ```

3. Rebuild and install with the local Node and Wails paths.

   ```sh
   /bin/zsh -lc "PATH=/Users/crp/.nvm/versions/node/v24.13.0/bin:/Users/crp/go/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin make install-darwin"
   ```

4. Verify the installed app bundle.

   ```sh
   ls -ld /Applications/aulycmail.app /Applications/aulycmail.app/Contents/MacOS/aulycmail
   codesign --verify --deep --strict --verbose=2 /Applications/aulycmail.app
   ```

## Last Confirmed Run

- Date/time: 2026-07-01 18:31 CST
- Running app PID before quit: none observed after quit request
- Quit result: no `aulycmail` main process was present before reinstall
- Install result: success
- Installed binary: `/Applications/aulycmail.app/Contents/MacOS/aulycmail`
- Installed timestamp observed: `Jul 1 18:31`
- Codesign verification: valid on disk; satisfies Designated Requirement

## Notes

- Running `make install-darwin` while the app is still open can leave the user testing an old process. Always quit first.
- The elevated/sanitized command environment may not include `npm`; include the explicit Node/npm path above.
- `wails build` regenerates `frontend/wailsjs/go/*` bindings. Review those generated diffs before committing.
