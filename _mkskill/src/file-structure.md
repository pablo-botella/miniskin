---
mkskill:
  pos: 140
  in: readme
---

## File structure example

```
content/
  content.miniskin.xml          # root: globals + bucket-list
  _skin/
    default.html                # skin layout
  app/
    _shared/
      header.html               # include fragment
    assets/
      assets.miniskin.xml       # resource-list for static files
      app.css
      app.js
    login_dialog/
      login.miniskin.xml        # resource-list + mockup-list
      login_mockup.html         # mockup source (mockup-export inside)
      signin_src.html           # source with front-matter + skin
      signin.html               # generated output (gitignored)
```

