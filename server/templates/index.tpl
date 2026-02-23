{{ define "main" }}
<script id="hotkey-data" type="application/json">
{{ .Config.Hotkeys.ToJSON }}
</script>
<input type="hidden" id="ws-url" value="{{ .WebSocketURL }}" />
<input type="hidden" id="csrf_token" value="{{ .CSRF }}" />
<input type="hidden" id="search-url" value="{{ .Config.App.SearchURL }}" />
<input type="hidden" id="open-results-on-new-tab" value="{{ .Config.App.OpenResultsOnNewTab }}" />
<input type="hidden" id="initial-query" value="{{ .Query }}" />
<div id="app"></div>
<script type="module" src="./static/js/dist/search.js" nonce="{{ .Nonce }}"></script>
{{ end }}
