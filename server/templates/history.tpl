{{define "main"}}
<div class="container full-width">
<h1>History</h1>
<table>
    <tr><th>Query</th><th>Result</th></tr>
    {{ range .History }}
    <tr><td><a href="/?q={{ .Query }}"><span class="success">{{ .Query }}</span></a></td><td><a href="{{ .URL }}">{{ .Title }}</a></td></tr>
    {{ end }}
</table>
</div>
{{ end }}
