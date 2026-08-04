import os

# Concept A: Monolithic Frame Break
concept_a_svg = '''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" fill="none">
  <path d="M 120 72 H 328 V 136 H 184 V 328 H 328 V 248 H 392 V 392 C 392 414 374 432 352 432 H 120 C 98 432 80 414 80 392 V 112 C 80 90 98 72 120 72 Z" fill="currentColor"/>
  <rect x="344" y="72" width="88" height="136" rx="16" fill="currentColor"/>
</svg>'''

# Concept B: Route Break (Slashed Portal)
concept_b_svg = '''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" fill="none">
  <path d="M 112 80 H 400 C 417.67 80 432 94.33 432 112 V 200 H 368 V 144 H 176 V 368 H 368 V 312 H 432 V 400 C 432 417.67 417.67 432 400 432 H 112 C 94.33 432 80 417.67 80 400 V 112 C 80 94.33 94.33 80 112 80 Z" fill="currentColor"/>
  <polygon points="240,240 448,240 448,288 240,288" fill="currentColor"/>
</svg>'''

# Concept C: UNBOUND Monogram (U-Break)
concept_c_svg = '''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" fill="none">
  <path d="M 96 80 H 168 V 344 C 168 357.25 178.75 368 192 368 H 320 C 333.25 368 344 357.25 344 344 V 240 H 416 V 344 C 416 397.02 373.02 440 320 440 H 192 C 138.98 440 96 397.02 96 344 V 80 Z" fill="currentColor"/>
  <rect x="344" y="80" width="72" height="112" rx="16" fill="currentColor"/>
</svg>'''

html_content = f'''<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="UTF-8">
<title>UNBOUND Logo Concept Evaluation</title>
<style>
  body {{ font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0e1117; color: #f5f5f5; margin: 0; padding: 40px; }}
  h1 {{ font-size: 24px; margin-bottom: 8px; font-weight: 600; letter-spacing: -0.5px; }}
  p.subtitle {{ color: #a3a3a3; font-size: 14px; margin-bottom: 32px; }}
  .grid {{ display: grid; grid-template-columns: repeat(3, 1fr); gap: 24px; }}
  .card {{ background: #161a22; border: 1px solid #262c36; border-radius: 16px; padding: 24px; }}
  .card-title {{ font-size: 18px; font-weight: 600; margin-bottom: 4px; }}
  .card-desc {{ font-size: 13px; color: #a3a3a3; margin-bottom: 20px; min-height: 40px; }}
  .preview-box {{ border-radius: 12px; padding: 20px; margin-bottom: 16px; display: flex; flex-direction: column; gap: 16px; }}
  .bg-dark {{ background: #08090b; color: #ffffff; border: 1px solid #242424; }}
  .bg-light {{ background: #f5f4f0; color: #141416; border: 1px solid #e2ded4; }}
  .bg-graphite {{ background: #121417; color: #f5f5f5; border: 1px solid #343a46; }}
  .row {{ display: flex; align-items: center; gap: 16px; }}
  .label {{ font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; opacity: 0.6; width: 70px; }}
  .sizes {{ display: flex; align-items: center; gap: 12px; }}
  .icon-wrapper {{ display: flex; align-items: center; justify-content: center; }}
  .wordmark {{ display: flex; align-items: center; gap: 10px; font-weight: 700; letter-spacing: 1px; font-size: 15px; }}
  .titlebar-preview {{ display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; border-radius: 8px; font-size: 12px; font-weight: 500; height: 32px; box-sizing: border-box; }}
  .app-icon-preview {{ width: 64px; height: 64px; border-radius: 14px; display: flex; align-items: center; justify-content: center; box-shadow: 0 4px 12px rgba(0,0,0,0.3); }}
</style>
</head>
<body>
<h1>UNBOUND Branding — Concept Evaluation (Precision Monochrome)</h1>
<p class="subtitle">Сравнение 3 концепций геометрического логотипа под тёмные, светлые и стальные темы</p>

<div class="grid">
  <!-- Concept A -->
  <div class="card">
    <div class="card-title">Concept A: Monolithic Frame Break</div>
    <div class="card-desc">Разомкнутый монолитный контур с угловым геометрическим разрывом и выходящим фрагментом.</div>
    
    <div class="preview-box bg-dark">
      <div class="row"><div class="label">Monolith</div>
        <div class="wordmark"><div style="width:24px;height:24px">{concept_a_svg}</div>UNBOUND</div>
      </div>
      <div class="row"><div class="label">Sizes</div>
        <div class="sizes">
          <div style="width:16px;height:16px">{concept_a_svg}</div>
          <div style="width:24px;height:24px">{concept_a_svg}</div>
          <div style="width:32px;height:32px">{concept_a_svg}</div>
          <div style="width:48px;height:48px">{concept_a_svg}</div>
        </div>
      </div>
      <div class="row"><div class="label">Titlebar</div>
        <div class="titlebar-preview bg-dark" style="width:100%">
          <div class="wordmark" style="font-size:12px"><div style="width:16px;height:16px">{concept_a_svg}</div>UNBOUND</div>
          <div style="font-size:10px;opacity:0.5">v0.2.3</div>
        </div>
      </div>
      <div class="row"><div class="label">App Icon</div>
        <div class="app-icon-preview" style="background:#141416; border:1px solid #2a2a2a">
          <div style="width:36px;height:36px;color:#ffffff">{concept_a_svg}</div>
        </div>
      </div>
    </div>

    <div class="preview-box bg-light">
      <div class="row"><div class="label">Paper</div>
        <div class="wordmark"><div style="width:24px;height:24px">{concept_a_svg}</div>UNBOUND</div>
      </div>
      <div class="row"><div class="label">App Icon</div>
        <div class="app-icon-preview" style="background:#ffffff; border:1px solid #d4d0c8">
          <div style="width:36px;height:36px;color:#141416">{concept_a_svg}</div>
        </div>
      </div>
    </div>
  </div>

  <!-- Concept B -->
  <div class="card">
    <div class="card-title">Concept B: Route Break</div>
    <div class="card-desc">Контурный портал с пересекающим векторным маршрутом обхода ограничений.</div>
    
    <div class="preview-box bg-dark">
      <div class="row"><div class="label">Monolith</div>
        <div class="wordmark"><div style="width:24px;height:24px">{concept_b_svg}</div>UNBOUND</div>
      </div>
      <div class="row"><div class="label">Sizes</div>
        <div class="sizes">
          <div style="width:16px;height:16px">{concept_b_svg}</div>
          <div style="width:24px;height:24px">{concept_b_svg}</div>
          <div style="width:32px;height:32px">{concept_b_svg}</div>
          <div style="width:48px;height:48px">{concept_b_svg}</div>
        </div>
      </div>
      <div class="row"><div class="label">Titlebar</div>
        <div class="titlebar-preview bg-dark" style="width:100%">
          <div class="wordmark" style="font-size:12px"><div style="width:16px;height:16px">{concept_b_svg}</div>UNBOUND</div>
          <div style="font-size:10px;opacity:0.5">v0.2.3</div>
        </div>
      </div>
      <div class="row"><div class="label">App Icon</div>
        <div class="app-icon-preview" style="background:#141416; border:1px solid #2a2a2a">
          <div style="width:36px;height:36px;color:#ffffff">{concept_b_svg}</div>
        </div>
      </div>
    </div>

    <div class="preview-box bg-light">
      <div class="row"><div class="label">Paper</div>
        <div class="wordmark"><div style="width:24px;height:24px">{concept_b_svg}</div>UNBOUND</div>
      </div>
      <div class="row"><div class="label">App Icon</div>
        <div class="app-icon-preview" style="background:#ffffff; border:1px solid #d4d0c8">
          <div style="width:36px;height:36px;color:#141416">{concept_b_svg}</div>
        </div>
      </div>
    </div>
  </div>

  <!-- Concept C -->
  <div class="card" style="border-color:#38bdf8">
    <div class="card-title">Concept C: UNBOUND Monogram (Selected)</div>
    <div class="card-desc">Монументальная монограмма U с геометрическим разрывом правого пилона (Break out).</div>
    
    <div class="preview-box bg-dark">
      <div class="row"><div class="label">Monolith</div>
        <div class="wordmark"><div style="width:24px;height:24px">{concept_c_svg}</div>UNBOUND</div>
      </div>
      <div class="row"><div class="label">Sizes</div>
        <div class="sizes">
          <div style="width:16px;height:16px">{concept_c_svg}</div>
          <div style="width:24px;height:24px">{concept_c_svg}</div>
          <div style="width:32px;height:32px">{concept_c_svg}</div>
          <div style="width:48px;height:48px">{concept_c_svg}</div>
        </div>
      </div>
      <div class="row"><div class="label">Titlebar</div>
        <div class="titlebar-preview bg-dark" style="width:100%">
          <div class="wordmark" style="font-size:12px"><div style="width:16px;height:16px">{concept_c_svg}</div>UNBOUND</div>
          <div style="font-size:10px;opacity:0.5">v0.2.3</div>
        </div>
      </div>
      <div class="row"><div class="label">App Icon</div>
        <div class="app-icon-preview" style="background:#141416; border:1px solid #2a2a2a">
          <div style="width:36px;height:36px;color:#ffffff">{concept_c_svg}</div>
        </div>
      </div>
    </div>

    <div class="preview-box bg-light">
      <div class="row"><div class="label">Paper</div>
        <div class="wordmark"><div style="width:24px;height:24px">{concept_c_svg}</div>UNBOUND</div>
      </div>
      <div class="row"><div class="label">App Icon</div>
        <div class="app-icon-preview" style="background:#ffffff; border:1px solid #d4d0c8">
          <div style="width:36px;height:36px;color:#141416">{concept_c_svg}</div>
        </div>
      </div>
    </div>
  </div>
</div>
</body>
</html>
'''

with open('/Users/kirill/Documents/unbound/docs/logo-concepts-preview.html', 'w') as f:
    f.write(html_content)

print("Generated logo concepts preview at docs/logo-concepts-preview.html")
