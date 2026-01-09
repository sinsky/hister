{{ define "base" -}}
<!doctype html>
<html lang='en'>
    <head>
        <meta charset='utf-8'>
        <title>Hister</title>
		<link rel="stylesheet" type="text/css" href="/static/style.css" />
          <link href="/favicon.ico" rel="icon shortcut" type="image/x-icon" />
    </head>
    <body>
        <header>
            <h1 class="menu-item"><img src="/static/logo.png" /> <a href='/'>Hister</a></h1>
            <a class="menu-item" href="/rules">Rules</a>
            <a class="menu-item" href="/add">Add</a>
            <a class="menu-item float-right" href="/help">Help</a>
        </header>
        <main>
            {{ if .Success }}
            <div class="container box success">
                <div class="header">{{ .Success }}</div>
                {{ if .SuccessMsg }}<div class="content">{{ .SuccessMsg }}</div>{{ end }}
            </div>
            {{ end }}
            {{ template "main" . }}
        </main>
        <footer>
            <a href='/about'>About</a> |
            <a href='https://github.com/asciimoo/hister/'>GitHub</a>
        </footer>
    </body>
</html>
{{- end -}}
