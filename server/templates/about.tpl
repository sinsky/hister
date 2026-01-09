{{define "main"}}
<div class="container full-width">
<h1>About Hister</h1>

<p>Hister is a web history management tool that provides blazing fast, content-based search for visited websites. Unlike traditional browser history that only searches URLs and titles, Hister indexes the full content of web pages you visit, enabling deep and meaningful search across your browsing history.</p>

<h2>Why Hister?</h2>

<p>Traditional browser history features are limited to basic keyword matching on URLs and page titles. Hister goes beyond these limitations by:

<ul>
	<li><b>Privacy-focused</b>: Keep your browsing history indexed locally on your machine</li>
	<li><b>Full-text indexing</b>: Search through the actual content of web pages you've visited</li>
	<li><b>Advanced search capabilities</b>: Utilize a powerful query language for precise results</li>
	<li><b>Smart organization</b>: Configure blacklist and priority rules for better control</li>
	<li><b>Efficient retrieval</b>: Use keyword aliases to quickly find content</li>
</ul>
</p>

<h2>Use Cases</h2>

<p>Hister is perfect for:

<ul>
	<li><b>Privacy protection</b>: Manage your data, access it without tracking</li>
	<li><b>Research</b>: Quickly find information from articles and documentation you've read</li>
	<li><b>Reference</b>: Locate code snippets, tutorials, or solutions you've visited before</li>
	<li><b>Knowledge management</b>: Build a searchable archive of your web browsing</li>
	<li><b>Productivity</b>: Spend less time searching for "that page I saw last week"</li>
</ul>
</p>

<h2>Technology</h2>

<p>Hister is built with Go and uses the Bleve search engine for lightning-fast full-text search capabilities. The application consists of:

<ul>
	<li>A local web server for search interface</li>
	<li>Browser extensions for <a href="https://chromewebstore.google.com/detail/hister/cciilamhchpmbdnniabclekddabkifhb">Chrome</a> and <a href="https://addons.mozilla.org/en-US/firefox/addon/hister/">Firefox</a> to automatically index visited pages</li>
</ul>
</p>

<h2>License</h2>

<p>Hister is free and open-source software licensed under AGPLv3.</p>

<h2>Community</h2>

<p>Found a bug or have a suggestion? Visit the <a href="https://github.com/asciimoo/hister/issues">issue tracker</a> on GitHub.</p>
</div>
{{ end }}
