![Logo](frontend/src/assets/images/logo-universal.png)

# aulycmail - An Open Source Lightweight E-Mail Client
Maintained by: @aulyc

![screenshot](docs/ss.png)


### ❓ Why?
---

Windows has Outlook

Mac has Mail

Linux has.....
 - Thunderbird - Clunky and too much legacy structure
 - Geary - Crippled by Gnome Online Accounts and search is unreliable
 - Mailspring - Electron...
 - Evolution - ... 1999

All are not necessarily always light on resource consumption...


### 👁️‍🗨️ Summary
---

A standalone lightweight e-mail client inspired by [Geary](https://wiki.gnome.org/Apps/Geary) focused on achieving the following goals:

- Resource Efficiency - Minimal CPU, RAM, and battery consumption
- Modern UX - Clean, intuitive interface with dark mode support
- Keyboard & Mouse Friendly - Full keyboard navigation with vim-style shortcuts
- Independence - No dependency on Gnome Online Accounts or other system services
- Search That Works - Basic search that actually finds your emails

### 🖥 OS Support
---

This checkout currently targets the local macOS desktop build:

- macOS

The upstream project has broader cross-platform history, but this repository's
active build/install flow is macOS-only.


### 🪶 Features
---

- Multiple Accounts
- Providers: (🧪 = NOT YET TESTED)
    - Generic IMAP/SMTP
    - GMail
    - Microsoft 365 / Outlook
    - Yahoo 
    - Proton Mail (via Proton Bridge)
    - iCloud Mail 
    - Mailfence
    - Murena
    - Fastmail 🧪
    - Zoho Mail 🧪
    - AOL Mail 🧪
    - GMX Mail 
    - Mail.com 🧪
    - Mailbox.org
- Unified Inbox (Color Code Accounts)
- Conversation Threads
- Basic Removal of Tracking Elements in Mail Content
- WYSIWYG Detachable Composer ([TipTap Editor](https://github.com/ueberdosis/tiptap))
- WYSIWYG Signatures ([TipTap Editor](https://github.com/ueberdosis/tiptap))
- Local contacts and autocomplete through the built-in Contacts extension
- Basic Local and IMAP Search
- Notification that brings focus to the e-mail when clicked
- Auto-Sync when system wakes from suspend
- Multiple color themes (More to come...)
- 1st party extension system with the following shipped:
    - Contacts (ALPHA) - Disabled by Default
- [Keyboard Shortcuts](docs/KEYBOARD_SHORTCUTS.md)


### 🚀 Installation
---

- [Official Installation Guide](https://github.com/aulyc/aulycmail)


### 📖 Documentation
---

- [Official Documentation](https://github.com/aulyc/aulycmail)


### ⚗️ Tech Stack
---

This application was built with [Wails](https://wails.io) + [Svelte](https://svelte.dev/).

Transparency Disclaimer: This project leaveraged Claude models heavily to implement.

aulycmail is CASA Tier 2 Certified by Google's preferred [authorized assessor](https://appdefensealliance.dev/casa/casa-assessors): [TAC Security](https://tacsecurity.com/)


### 🗞 News & Announcments
---

- 2026-03-11 ~ Microsoft Verified
- 2026-04-16 ~ CASA Tier 2 Certified
- 2025-04-25 ~ Google Verified


### 🧑🏻‍💻 Roadmap
---

Confirmed future features:

- Post quantum ready encryption
- Add Mailfence and Startmail templates in add account flow for easier setup

Potential features in the future:

- Customizable shortcut keys
- Advance Search
- AI Assisted Composition (Ollama)


### 🏷️ Changelog
---

[CHANGELOG.md](CHANGELOG.md)


### 🔨 Contributing
---

Please see [CONTRIBUTING.md](CONTRIBUTING.md)


### 🙏 Issue Contributors
---

aulycmail is largely driven by community feedback. Big thanks to the following non-exhaustive list of contributors who submitted issues which led to meaningful improvements we all now enjoy. This project would not be the same without them!

<table>
  <tr>
    <td align="center">
      <a href="https://github.com/keithvassallomt">
        <img src="https://github.com/keithvassallomt.png" width="80"><br>
        <sub><b>keithvassallomt</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Akeithvassallomt+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>19 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/The-Nyla">
        <img src="https://github.com/The-Nyla.png" width="80"><br>
        <sub><b>The-Nyla</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3AThe-Nyla+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>6 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/shahiljain">
        <img src="https://github.com/shahiljain.png" width="80"><br>
        <sub><b>shahiljain</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Ashahiljain+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>4 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/isorropisths">
        <img src="https://github.com/isorropisths.png" width="80"><br>
        <sub><b>isorropisths</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Aisorropisths+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>4 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/arnauda-gh">
        <img src="https://github.com/arnauda-gh.png" width="80"><br>
        <sub><b>arnauda-gh</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Aarnauda-gh+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>4 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/lorduskordus">
        <img src="https://github.com/lorduskordus.png" width="80"><br>
        <sub><b>lorduskordus</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Alorduskordus+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>3 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/jeremy-niles">
        <img src="https://github.com/jeremy-niles.png" width="80"><br>
        <sub><b>jeremy-niles</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Ajeremy-niles+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>2 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/clintre">
        <img src="https://github.com/clintre.png" width="80"><br>
        <sub><b>clintre</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Aclintre+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>2 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/SonGokuSSJ">
        <img src="https://github.com/SonGokuSSJ.png" width="80"><br>
        <sub><b>SonGokuSSJ</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3ASonGokuSSJ+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>2 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/lawmanuk">
        <img src="https://github.com/lawmanuk.png" width="80"><br>
        <sub><b>lawmanuk</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Alawmanuk+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/CDrummond">
        <img src="https://github.com/CDrummond.png" width="80"><br>
        <sub><b>CDrummond</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3ACDrummond+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/woolkingx">
        <img src="https://github.com/woolkingx.png" width="80"><br>
        <sub><b>woolkingx</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Awoolkingx+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/kimusan">
        <img src="https://github.com/kimusan.png" width="80"><br>
        <sub><b>kimusan</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Akimusan+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/Kartoffelbauer">
        <img src="https://github.com/Kartoffelbauer.png" width="80"><br>
        <sub><b>Kartoffelbauer</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3AKartoffelbauer+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/alfureu">
        <img src="https://github.com/alfureu.png" width="80"><br>
        <sub><b>alfureu</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Aalfureu+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/Gerti1972">
        <img src="https://github.com/Gerti1972.png" width="80"><br>
        <sub><b>Gerti1972</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3AGerti1972+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/piresio">
        <img src="https://github.com/piresio.png" width="80"><br>
        <sub><b>piresio</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Apiresio+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/cvdmint">
        <img src="https://github.com/cvdmint.png" width="80"><br>
        <sub><b>cvdmint</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Acvdmint+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/AdamHasma">
        <img src="https://github.com/AdamHasma.png" width="80"><br>
        <sub><b>AdamHasma</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3AAdamHasma+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/pj398">
        <img src="https://github.com/pj398.png" width="80"><br>
        <sub><b>pj398</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Apj398+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/CreateWebNZ">
        <img src="https://github.com/CreateWebNZ.png" width="80"><br>
        <sub><b>CreateWebNZ</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3ACreateWebNZ+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/initsuj">
        <img src="https://github.com/initsuj.png" width="80"><br>
        <sub><b>initsuj</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Ainitsuj+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/HugoTheBoss">
        <img src="https://github.com/HugoTheBoss.png" width="80"><br>
        <sub><b>HugoTheBoss</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3AHugoTheBoss+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/makzumi">
        <img src="https://github.com/makzumi.png" width="80"><br>
        <sub><b>makzumi</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Amakzumi+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/mmzim05">
        <img src="https://github.com/mmzim05.png" width="80"><br>
        <sub><b>mmzim05</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Ammzim05+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/robert0815">
        <img src="https://github.com/robert0815.png" width="80"><br>
        <sub><b>robert0815</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Arobert0815+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/Olivetti">
        <img src="https://github.com/Olivetti.png" width="80"><br>
        <sub><b>Olivetti</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3AOlivetti+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/martink1337">
        <img src="https://github.com/martink1337.png" width="80"><br>
        <sub><b>martink1337</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Amartink1337+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/in4matix">
        <img src="https://github.com/in4matix.png" width="80"><br>
        <sub><b>in4matix</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Ain4matix+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/srabette">
        <img src="https://github.com/srabette.png" width="80"><br>
        <sub><b>srabette</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Asrabette+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/Infiniti151">
        <img src="https://github.com/Infiniti151.png" width="80"><br>
        <sub><b>Infiniti151</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3AInfiniti151+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/bjacobs39">
        <img src="https://github.com/bjacobs39.png" width="80"><br>
        <sub><b>bjacobs39</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Abjacobs39+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/justin-lavelle">
        <img src="https://github.com/justin-lavelle.png" width="80"><br>
        <sub><b>justin-lavelle</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Ajustin-lavelle+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/diederikh">
        <img src="https://github.com/diederikh.png" width="80"><br>
        <sub><b>diederikh</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Adiederikh+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/frian92">
        <img src="https://github.com/frian92.png" width="80"><br>
        <sub><b>frian92</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Afrian92+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/Dragonsong3k">
        <img src="https://github.com/Dragonsong3k.png" width="80"><br>
        <sub><b>Dragonsong3k</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3ADragonsong3k+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/TheEmi">
        <img src="https://github.com/TheEmi.png" width="80"><br>
        <sub><b>TheEmi</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3ATheEmi+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/ai-mind">
        <img src="https://github.com/ai-mind.png" width="80"><br>
        <sub><b>ai-mind</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Aai-mind+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/yuukiw">
        <img src="https://github.com/yuukiw.png" width="80"><br>
        <sub><b>yuukiw</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Ayuukiw+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/budfy">
        <img src="https://github.com/budfy.png" width="80"><br>
        <sub><b>budfy</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Abudfy+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/Arvid-ctrl">
        <img src="https://github.com/Arvid-ctrl.png" width="80"><br>
        <sub><b>Arvid-ctrl</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3AArvid-ctrl+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/arodier">
        <img src="https://github.com/arodier.png" width="80"><br>
        <sub><b>arodier</b></sub>
      </a><br>
      <a href="https://github.com/aulyc/aulycmail/issues?q=is%3Aissue+is%3Aclosed+author%3Aarodier+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
</table>

*Last Updated: 2026-05-27 | Generated by gitrix


### 🌐 Translation Contributors

Special thanks to translation contributors for making aulycmail more accessible:


<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/lorduskordus">
        <img src="https://github.com/lorduskordus.png" width="80"><br>
        <sub><b>lorduskordus</b></sub>
      </a><br>
      <sub>Čeština (cs)</sub><br>
      <sub>&nbsp;</sub>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/Gerti1972">
        <img src="https://github.com/Gerti1972.png" width="80"><br>
        <sub><b>Gerti1972</b></sub>
      </a><br>
      <sub>Deutsch (de)</sub><br>
      <sub>&nbsp;</sub>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/dev-inside">
        <img src="https://github.com/dev-inside.png" width="80"><br>
        <sub><b>dev-inside</b></sub>
      </a><br>
      <sub>Deutsch (de)</sub><br>
      <sup><small><em>reviewer</em></small></sup>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/StefanSchroeder">
        <img src="https://github.com/StefanSchroeder.png" width="80"><br>
        <sub><b>StefanSchroeder</b></sub>
      </a><br>
      <sub>Deutsch (de)</sub><br>
      <sup><small><em>reviewer</em></small></sup>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/freemans32">
        <img src="https://github.com/freemans32.png" width="80"><br>
        <sub><b>freemans32</b></sub>
      </a><br>
      <sub>Français (fr)</sub><br>
      <sub>&nbsp;</sub>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/YacineBoussoufa">
        <img src="https://github.com/YacineBoussoufa.png" width="80"><br>
        <sub><b>YacineBoussoufa</b></sub>
      </a><br>
      <sub>Italiano (it)</sub><br>
      <sub>&nbsp;</sub>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/dexblasnoot">
        <img src="https://github.com/dexblasnoot.png" width="80"><br>
        <sub><b>dexblasnoot</b></sub>
      </a><br>
      <sub>Norsk Bokmål (nb)</sub><br>
      <sub>&nbsp;</sub>
    </td>
  </tr>
</table>

<br clear="left">


### 📑 Terms of Use & Privacy Policy
---

- [Terms of Use](docs/TERMS.md)
- [Privacy Policy](docs/PRIVACY.md)
