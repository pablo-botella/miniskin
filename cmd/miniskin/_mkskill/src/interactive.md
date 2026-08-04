---
mkskill:
  pos: 50
---

## Interactive use

Day-to-day commands run from the terminal. The examples below assume you run miniskin from the project root and your assets live in a `content/` subdirectory:

```
miniskin run -content content
miniskin run -content content -v
miniskin deps -content content
miniskin mockup negative -src mockup_login.html -dst login_negative.html
miniskin -generate-agent-docs -dst .cursor/rules/miniskin.mdc -force
miniskin combine content/app
miniskin split content/app/app.miniskin.xml
```

