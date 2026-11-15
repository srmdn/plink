# Article editor bundle

The application embeds `web/js/article-editor.js` in its Go binary. After changing
`editor.js`, run `npm ci` followed by `npm run build` in this directory and include
the rebuilt bundle with the source change. The browser editor stores article body
content as Markdown. Unsupported Markdown opens in source mode to avoid loss.

The corresponding bundled dependency license notices are in
`web/js/article-editor-licenses.txt`. Update them when changing bundled dependencies.
