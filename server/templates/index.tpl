{{ define "main" }}
<div class="sticky">
    <div class="search text-center">
        <input type="text" autofocus id="search" />
        <input type="text" disabled id="autocomplete" value="Search..."/>
        <div id="ws-status" class="ws-status" title="Websocket disconnected"></div>
    </div>
</div>
<button id="hotkey-button" class="hotkeys-button" title="Hotkeys (?)">
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <rect x="2" y="4" width="20" height="16" rx="2"/>
        <path d="M6 8h.01M10 8h.01M14 8h.01M18 8h.01M8 12h.01M12 12h.01M16 12h.01M7 16h10"/>
    </svg>
</button>
<div class="container">
    <div id="results"></div>
</div>
<template id="result">
    <div class="result">
        <div class="result-title"><img><a></a></div>
        <span class="result-url"></span><span class="action-button"><svg focusable="false" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path  fill="#95a5a6" d="M12 8c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z"></path></svg></span><span class="added"></span> <a class="readable">view</a>
        <p class="result-content"></p>
    </div>
</template>
<template id="results-header">
    <div class="results-header">
        <div class="duration float-right"></div>
        <div>Total number of results: <b class="results-num"></b></div>
        <div class="expanded-query"></div>
        <div class="export-buttons">
            Export: <a class="export-json">JSON</a> | <a class="export-csv">CSV</a> | <a class="export-rss">RSS</a>
        </div>
    </div>
</template>
<template id="tips">
<div class="text-center">
    <h3>Tip</h3>
    <p class="content"></p>
</div>
</template>
<template id="result-actions">
<div class="actions bordered padded mt-1">
    <a class="close float-right">close</a>
    Prioritize this result for the following query:<br />
    <input type="text" class="action-query" placeholder="Query.." />
    <button class="save">Save</button><br />
    <button class="delete error">Delete this result</button>
</div>
</template>
<template id="priority-actions">
<div class="actions bordered padded mt-1">
    <a class="close float-right">close</a>
    <button class="delete error">Delete this priority result</button>
</div>
</template>
<template id="success">
<p class="success">
    <b>Success!</b> <span class="message"></span>
</p>
</template>
<template id="error">
<p class="error">
    <b>Error!</b> <span class="message"></span>
</p>
</template>
<template id="popup">
<div class="popup-wrapper">
    <div class="popup container">
        <div class="float-right"><a class="popup-close">x</a></div>
        <div class="popup-header"></div>
        <div class="popup-content"></div>
    </div>
</div>
</template>
<template id="hotkey">
    <div class="hotkey">
        <div><kbd></kbd></div>
        <span></span>
    </div>
</template>
<script id="hotkey-data" type="application/json">
{{ .Config.Hotkeys.ToJSON }}
</script>
<input type="hidden" id="ws-url" value="{{ .Config.WebSocketURL }}" />
<input type="hidden" id="csrf_token" value="{{ .CSRF }}" />
<input type="hidden" id="search-url" value="{{ .Config.App.SearchURL }}" />
<input type="hidden" id="open-results-on-new-tab" value="{{ .Config.App.OpenResultsOnNewTab }}" />
<script src="static/js/search.js" nonce="{{ .Nonce }}"></script>
{{ end }}
