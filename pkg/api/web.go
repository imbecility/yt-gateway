package api

var tmpl = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>YT-Gateway</title>
    <style>
        :root { --bg: #121212; --card: #1e1e1e; --text: #e0e0e0; --accent: #ff4444; }
        body { background: var(--bg); color: var(--text); font-family: system-ui, sans-serif; display: grid; place-items: center; min-height: 100vh; margin: 0; }
        .container { background: var(--card); padding: 2rem; border-radius: 12px; box-shadow: 0 10px 30px rgba(0,0,0,0.5); width: 90%; max-width: 400px; text-align: center; }
        h1 { margin: 0 0 1rem; font-size: 1.5rem; color: var(--accent); }
        input { width: 100%; padding: 12px; margin: 10px 0; border: 1px solid #333; border-radius: 6px; background: #252525; color: #fff; box-sizing: border-box; outline: none; }
        input:focus { border-color: var(--accent); }
        button { width: 100%; padding: 12px; border: none; border-radius: 6px; background: var(--accent); color: white; font-weight: bold; cursor: pointer; transition: 0.2s; }
        button:hover { opacity: 0.9; }
        button:disabled { background: #555; cursor: not-allowed; }
        #result { margin-top: 20px; line-height: 1.6; word-break: break-word; }
        a { display: inline-block; margin: 5px; color: #4ea8de; text-decoration: none; border: 1px solid #4ea8de; padding: 5px 10px; border-radius: 4px; font-size: 0.9rem; }
        a:hover { background: #4ea8de; color: #fff; }
        .error { color: var(--accent); font-size: 0.9rem; }
    </style>
</head>
<body>
    <div class="container">
        <h1>YouTube Gateway</h1>
        <form id="dlForm">
            <input type="url" id="url" placeholder="Paste YouTube URL..." required>
            <button type="submit" id="btn">Get Video Links</button>
        </form>
        <div id="result"></div>
    </div>

    <script>
        const f = document.getElementById('dlForm'), 
              r = document.getElementById('result'),
              b = document.getElementById('btn');

        f.onsubmit = async (e) => {
            e.preventDefault();
            b.disabled = true;
            r.innerHTML = '⏳ Processing...';
            
            try {
                const resp = await fetch('/api/download', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({url: document.getElementById('url').value})
                });
                const data = await resp.json();
                
                if (!data.success) throw new Error(data.error);
                let html = '<div style="font-weight:bold;margin-bottom:10px">' + data.title + '</div>';
                if (data.direct_url) html += '<a href="' + data.direct_url + '" target="_blank">📥 Direct Link</a>';
                if (data.stream_url) html += '<a href="' + data.stream_url + '" target="_blank">🎬 Stream Link</a>';
                r.innerHTML = html;

            } catch (err) {
                r.innerHTML = '<div class="error">❌ ' + err.message + '</div>';
            } finally {
                b.disabled = false;
            }
        };
    </script>
</body>
</html>
`
