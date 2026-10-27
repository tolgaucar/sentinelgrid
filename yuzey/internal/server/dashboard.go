package server

import "net/http"

// dashboardHTML — YÜZEY taktiksel izleme ekranı (Palantir Gotham tarzı)
const dashboardHTML = `<!DOCTYPE html>
<html lang="tr">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>YÜZEY — ÖTÜKEN-MATRİS Taktiksel İzleme</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{
  --bg:#060a12;--card:#0c1220;--card2:#111a2d;--surface:#162036;--border:#1c2a45;
  --text:#d4dced;--dim:#5a6d8a;--muted:#384860;
  --green:#00e68a;--red:#ff4d6a;--yellow:#ffc233;--cyan:#00c2ff;--blue:#2d7fff;
  --purple:#9b6dff;--orange:#ff8533;--pink:#ff5c93;
  --friendly:#2d7fff;--enemy:#ff4d6a;--neutral:#ffc233;--unknown:#5a6d8a;
}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.5}}
@keyframes slideIn{from{transform:translateX(100%);opacity:0}to{transform:translateX(0);opacity:1}}
@keyframes fadeIn{from{opacity:0}to{opacity:1}}
body{font-family:'Inter','SF Pro Text',-apple-system,system-ui,sans-serif;background:var(--bg);color:var(--text);overflow:hidden;height:100vh;-webkit-font-smoothing:antialiased}
.app{display:flex;flex-direction:column;height:100vh}
.top-bar{display:flex;align-items:center;justify-content:space-between;padding:0 20px;height:48px;flex-shrink:0;
  background:linear-gradient(180deg,rgba(12,18,32,.98),rgba(6,10,18,.95));
  border-bottom:1px solid var(--border);backdrop-filter:blur(10px);z-index:100;position:relative}
.top-bar::after{content:'';position:absolute;bottom:0;left:0;right:0;height:1px;
  background:linear-gradient(90deg,transparent,var(--cyan),transparent);opacity:.3}
.logo{display:flex;align-items:center;gap:10px}
.logo-icon{width:28px;height:28px;border:2px solid var(--cyan);border-radius:6px;display:flex;align-items:center;
  justify-content:center;font-size:14px;background:rgba(0,194,255,.08);color:var(--cyan)}
.logo h1{font-size:13px;font-weight:600;color:var(--text);letter-spacing:1.5px}
.logo h1 span{color:var(--cyan);font-weight:700}
.breadcrumb{font-size:12px;color:var(--dim);font-weight:600;letter-spacing:1px;text-transform:uppercase}
.top-stats{display:flex;gap:16px;align-items:center}
.top-stat{display:flex;align-items:center;gap:5px;font-size:11px;color:var(--dim)}
.top-stat b{color:var(--text);font-size:12px;font-weight:600;font-variant-numeric:tabular-nums}
.conn-badge{display:flex;align-items:center;gap:5px;padding:3px 10px;border-radius:12px;font-size:10px;font-weight:600}
.conn-badge.ok{background:rgba(0,230,138,.1);color:var(--green);border:1px solid rgba(0,230,138,.2)}
.conn-badge.err{background:rgba(255,77,106,.1);color:var(--red);border:1px solid rgba(255,77,106,.2)}
.conn-dot{width:6px;height:6px;border-radius:50%;display:inline-block}
.conn-badge.ok .conn-dot{background:var(--green);box-shadow:0 0 8px var(--green)}
.conn-badge.err .conn-dot{background:var(--red);animation:pulse 1.5s infinite}
.clock{font-size:11px;color:var(--dim);font-variant-numeric:tabular-nums;font-weight:500}
.app-body{display:flex;flex:1;overflow:hidden}
.sidebar{width:60px;flex-shrink:0;background:var(--card);border-right:1px solid var(--border);
  transition:width .2s ease;overflow:hidden;z-index:50;display:flex;flex-direction:column}
.sidebar:hover{width:200px}
.sidebar-nav{flex:1;padding:8px 0}
.nav-item{display:flex;align-items:center;gap:12px;height:44px;padding:0 18px;cursor:pointer;
  transition:all .15s;border-left:3px solid transparent;white-space:nowrap;color:var(--dim);font-size:11px;font-weight:600}
.nav-item:hover{background:var(--surface);color:var(--text)}
.nav-item.active{border-left-color:var(--cyan);background:rgba(0,194,255,.06);color:var(--cyan)}
.nav-icon{font-size:18px;min-width:24px;text-align:center}
.nav-label{opacity:0;transition:opacity .15s}
.sidebar:hover .nav-label{opacity:1}
.content{flex:1;overflow:hidden;position:relative}
.page{display:none;width:100%;height:100%;overflow-y:auto;overflow-x:hidden}
.page.active{display:flex;flex-direction:column}
.page-fullscreen.active{display:block;overflow:hidden}
.kpi-row{display:flex;gap:12px;padding:16px 20px;flex-shrink:0}
.kpi-card{flex:1;background:var(--card);border:1px solid var(--border);border-radius:8px;padding:14px 16px;
  transition:all .15s;position:relative;overflow:hidden}
.kpi-card:hover{border-color:rgba(0,194,255,.3);transform:translateY(-1px)}
.kpi-val{font-size:26px;font-weight:800;font-variant-numeric:tabular-nums;line-height:1}
.kpi-lbl{font-size:9px;color:var(--dim);text-transform:uppercase;letter-spacing:.5px;margin-top:4px}
.kpi-sub{font-size:9px;color:var(--muted);margin-top:2px}
.overview-grid{display:grid;grid-template-columns:3fr 2fr;gap:16px;padding:0 20px 16px;flex-shrink:0}
.overview-grid2{display:grid;grid-template-columns:1fr 1fr;gap:16px;padding:0 20px 16px;flex-shrink:0}
.panel-box{background:var(--card);border:1px solid var(--border);border-radius:8px;overflow:hidden;display:flex;flex-direction:column}
.panel-header{padding:10px 14px;border-bottom:1px solid var(--border);display:flex;align-items:center;
  justify-content:space-between;background:var(--card2)}
.panel-title{font-size:10px;font-weight:700;color:var(--dim);text-transform:uppercase;letter-spacing:1px}
.panel-badge{font-size:9px;padding:2px 8px;border-radius:8px;background:var(--surface);color:var(--dim)}
.panel-body{flex:1;overflow-y:auto;padding:10px 14px}
.tactical-wrap{position:relative;width:100%;height:100%}
.tactical-wrap canvas{width:100%;height:100%;display:block}
.float-panel{position:absolute;background:rgba(12,18,32,.92);border:1px solid var(--border);
  border-radius:8px;backdrop-filter:blur(12px);z-index:10}
.float-controls{top:12px;right:12px;display:flex;gap:4px;padding:4px}
.float-layers{bottom:12px;left:12px;padding:8px;max-height:200px;overflow-y:auto;min-width:180px}
.float-entities{bottom:12px;right:12px;width:220px;max-height:240px;overflow-y:auto;padding:8px}
.float-detail{top:0;right:0;width:360px;height:100%;border-radius:0;display:none;flex-direction:column;
  overflow:hidden;animation:slideIn .2s ease-out;box-shadow:-8px 0 32px rgba(0,0,0,.3)}
.float-detail.open{display:flex}
.map-btn{background:rgba(12,18,32,.92);border:1px solid var(--border);color:var(--dim);padding:6px 12px;
  font-size:10px;font-family:inherit;cursor:pointer;border-radius:6px;transition:all .15s;font-weight:600}
.map-btn:hover{border-color:var(--cyan);color:var(--text)}
.map-btn.active{background:rgba(0,194,255,.15);color:var(--cyan);border-color:var(--cyan)}
.overlay-info{position:absolute;bottom:10px;left:200px;background:rgba(6,10,18,.85);padding:6px 12px;
  border-radius:6px;font-size:10px;color:var(--dim);pointer-events:none;border:1px solid var(--border);
  font-variant-numeric:tabular-nums;z-index:10}
.map-tooltip{position:fixed;pointer-events:none;background:rgba(12,18,32,.95);border:1px solid var(--border);
  border-radius:8px;padding:10px 12px;font-size:10px;z-index:200;min-width:180px;max-width:240px;
  backdrop-filter:blur(12px);box-shadow:0 8px 32px rgba(0,0,0,.5);display:none;animation:fadeIn .1s}
.tt-header{display:flex;align-items:center;gap:6px;margin-bottom:6px;padding-bottom:6px;border-bottom:1px solid var(--border)}
.tt-icon{font-size:16px}.tt-name{font-weight:700;font-size:11px}
.tt-faction{font-size:9px;padding:1px 6px;border-radius:8px;font-weight:600}
.tt-row{display:flex;justify-content:space-between;padding:2px 0;color:var(--dim)}
.tt-val{color:var(--text);font-weight:500;font-variant-numeric:tabular-nums}
.ed-header{padding:14px 16px;border-bottom:1px solid var(--border);display:flex;align-items:center;gap:10px;background:var(--card2)}
.ed-close{width:28px;height:28px;border-radius:6px;border:1px solid var(--border);background:none;color:var(--dim);
  cursor:pointer;font-size:14px;display:flex;align-items:center;justify-content:center;transition:all .15s}
.ed-close:hover{background:var(--surface);color:var(--red);border-color:var(--red)}
.ed-icon{font-size:24px}.ed-title{flex:1}
.ed-title h3{font-size:13px;font-weight:700;margin-bottom:2px}
.ed-title .ed-sub{font-size:10px;color:var(--dim)}
.ed-body{flex:1;overflow-y:auto;padding:12px 16px}
.ed-section{margin-bottom:14px}
.ed-section-title{font-size:9px;font-weight:700;color:var(--dim);text-transform:uppercase;letter-spacing:1px;
  margin-bottom:6px;padding-bottom:4px;border-bottom:1px solid var(--border)}
.ed-grid{display:grid;grid-template-columns:1fr 1fr;gap:6px}
.ed-field{background:var(--surface);border-radius:6px;padding:8px 10px;border:1px solid var(--border)}
.ed-field .ed-f-val{font-size:14px;font-weight:700;color:var(--text);font-variant-numeric:tabular-nums}
.ed-field .ed-f-lbl{font-size:8px;color:var(--dim);text-transform:uppercase;letter-spacing:.5px;margin-top:2px}
.table-toolbar{display:flex;align-items:center;gap:10px;padding:12px 20px;background:var(--card);
  border-bottom:1px solid var(--border);flex-wrap:wrap;flex-shrink:0}
.table-search{flex:1;min-width:200px;padding:8px 12px;background:var(--surface);border:1px solid var(--border);
  border-radius:6px;color:var(--text);font-size:12px;font-family:inherit;outline:none}
.table-search:focus{border-color:var(--cyan)}
.table-search::placeholder{color:var(--muted)}
.filter-group{display:flex;gap:4px}
.filter-btn{padding:4px 10px;font-size:10px;font-weight:600;border-radius:4px;border:1px solid var(--border);
  background:var(--surface);color:var(--dim);cursor:pointer;transition:all .15s;font-family:inherit}
.filter-btn:hover{border-color:var(--cyan);color:var(--text)}
.filter-btn.active{background:rgba(0,194,255,.15);color:var(--cyan);border-color:var(--cyan)}
.data-table{width:100%;border-collapse:collapse}
.data-table th{position:sticky;top:0;background:var(--card2);padding:8px 10px;font-size:10px;font-weight:700;
  color:var(--dim);text-transform:uppercase;letter-spacing:.5px;border-bottom:1px solid var(--border);
  cursor:pointer;white-space:nowrap;text-align:left;user-select:none}
.data-table th:hover{color:var(--cyan)}
.sort-arrow{font-size:8px;margin-left:4px}
.data-table td{padding:6px 10px;font-size:11px;border-bottom:1px solid rgba(28,42,69,.3);
  white-space:nowrap;font-variant-numeric:tabular-nums}
.data-table tr{transition:background .1s;cursor:pointer}
.data-table tr:hover{background:var(--surface)}
.data-table tr.selected{background:rgba(0,194,255,.06)}
.hp-mini{width:40px;height:4px;border-radius:2px;background:var(--border);display:inline-block;vertical-align:middle;overflow:hidden}
.hp-mini-fill{height:100%;border-radius:2px;display:block}
.events-toolbar{display:flex;align-items:center;gap:10px;padding:12px 20px;background:var(--card);
  border-bottom:1px solid var(--border);flex-wrap:wrap;flex-shrink:0}
.sev-pills{display:flex;gap:4px}
.sev-pill{padding:4px 12px;font-size:10px;font-weight:600;border-radius:12px;border:1px solid var(--border);
  background:var(--surface);color:var(--dim);cursor:pointer;transition:all .15s;font-family:inherit}
.sev-pill:hover{border-color:var(--cyan)}
.sev-pill.active{border-color:var(--cyan);color:var(--cyan);background:rgba(0,194,255,.08)}
.sev-counts{display:flex;gap:12px;margin-left:auto;font-size:10px;color:var(--dim)}
.event-card{padding:12px 20px;border-bottom:1px solid rgba(28,42,69,.3);transition:background .15s}
.event-card:hover{background:rgba(22,32,54,.5)}
.event-card-header{display:flex;align-items:center;gap:8px;margin-bottom:4px}
.sev-dot{width:8px;height:8px;border-radius:50%;flex-shrink:0}
.sev-dot.info{background:var(--cyan)}.sev-dot.warning{background:var(--yellow)}
.sev-dot.critical{background:var(--red);animation:pulse 1.5s infinite}
.sev-dot.success{background:var(--green)}
.event-title{font-size:12px;font-weight:500;flex:1}
.event-time{font-size:10px;color:var(--muted);font-variant-numeric:tabular-nums}
.event-source{display:inline-block;font-size:9px;padding:2px 8px;border-radius:4px;background:var(--surface);color:var(--dim);margin-right:6px}
.event-detail{font-size:11px;color:var(--dim);margin-top:4px;padding-left:16px}
.event-sev-label{font-size:8px;font-weight:700;padding:2px 6px;border-radius:3px;letter-spacing:.5px;flex-shrink:0}
.sev-label-critical{background:rgba(255,77,106,.15);color:var(--red)}
.sev-label-warning{background:rgba(255,194,51,.15);color:var(--yellow)}
.sev-label-info{background:rgba(102,217,239,.1);color:var(--cyan)}
.module-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:16px;padding:20px}
.mod-card{background:var(--card);border:1px solid var(--border);border-radius:8px;overflow:hidden;transition:all .15s}
.mod-card:hover{border-color:rgba(0,194,255,.3);transform:translateY(-2px)}
.mod-card-header{padding:12px 14px;display:flex;align-items:center;gap:10px;border-bottom:1px solid var(--border);background:var(--card2)}
.mod-card-icon{font-size:24px}.mod-card-info{flex:1}
.mod-card-name{font-size:13px;font-weight:700}
.mod-card-desc{font-size:9px;color:var(--dim);margin-top:2px}
.mod-card-status{width:10px;height:10px;border-radius:50%}
.mod-card-status.alive{background:var(--green);box-shadow:0 0 8px rgba(0,230,138,.4)}
.mod-card-status.dead{background:var(--red);box-shadow:0 0 8px rgba(255,77,106,.4)}
.mod-card-body{padding:12px 14px}
.mod-card-stats{display:grid;grid-template-columns:1fr 1fr;gap:8px;margin-bottom:10px}
.mod-stat{background:var(--surface);border-radius:6px;padding:8px 10px;border:1px solid var(--border)}
.mod-stat .val{font-size:16px;font-weight:700;font-variant-numeric:tabular-nums}
.mod-stat .lbl{font-size:8px;color:var(--dim);text-transform:uppercase;letter-spacing:.5px;margin-top:2px}
.spark-chart{width:100%;height:40px}
.faction-bar{display:flex;height:6px;border-radius:3px;overflow:hidden;gap:1px}
.faction-bar div{height:100%;border-radius:2px;transition:width .3s}
.faction-legend{display:flex;gap:10px;font-size:9px;color:var(--dim);margin-top:6px;flex-wrap:wrap}
.faction-legend span{display:flex;align-items:center;gap:3px}
.faction-dot{width:6px;height:6px;border-radius:50%;display:inline-block}
.layer-row{display:flex;align-items:center;gap:8px;padding:4px 6px;border-radius:5px;cursor:pointer;transition:all .15s;font-size:11px}
.layer-row:hover{background:var(--surface)}
.layer-check{width:16px;height:16px;border:1.5px solid var(--muted);border-radius:3px;display:flex;
  align-items:center;justify-content:center;transition:all .15s;font-size:9px;flex-shrink:0}
.layer-check.on{background:var(--cyan);border-color:var(--cyan);color:#000;font-weight:bold}
.mini-ent{display:flex;align-items:center;gap:6px;padding:4px 6px;border-radius:4px;cursor:pointer;
  transition:all .1s;font-size:10px;border:1px solid transparent}
.mini-ent:hover{background:var(--surface)}
.mini-ent.selected{background:rgba(0,194,255,.1);border-color:rgba(0,194,255,.2)}
.mod-grid-sm{display:grid;grid-template-columns:repeat(3,1fr);gap:6px}
.mod-grid-sm-item{background:var(--surface);border-radius:6px;padding:8px;border:1px solid var(--border);text-align:center;transition:all .15s}
.mod-grid-sm-item:hover{border-color:rgba(0,194,255,.3)}
::-webkit-scrollbar{width:4px}::-webkit-scrollbar-track{background:transparent}
::-webkit-scrollbar-thumb{background:var(--muted);border-radius:2px}
::-webkit-scrollbar-thumb:hover{background:var(--dim)}
</style>
</head>
<body>
<div class="app">
  <div class="top-bar">
    <div class="logo"><div class="logo-icon">&#9672;</div><h1><span>Y&#220;ZEY</span> &#8212; TAKT&#304;KSEL &#304;ZLEME S&#304;STEM&#304;</h1></div>
    <div class="breadcrumb" id="breadcrumb">GENEL BAKI&#350;</div>
    <div class="top-stats">
      <span class="clock" id="clockEl">00:00:00</span>
      <div class="conn-badge ok" id="connBadge"><span class="conn-dot"></span><span id="connText">Ba&#287;lan&#305;yor</span></div>
      <div class="top-stat">Mod&#252;ller <b><span id="modAlive">0</span>/<span id="modTotal">0</span></b></div>
      <div class="top-stat">Varl&#305;klar <b id="entCount">0</b></div>
      <div class="top-stat">Olaylar <b id="evtCount">0</b></div>
    </div>
  </div>
  <div class="app-body">
    <div class="sidebar">
      <div class="sidebar-nav">
        <div class="nav-item active" onclick="switchPage('overview')"><span class="nav-icon">&#128202;</span><span class="nav-label">GENEL BAKI&#350;</span></div>
        <div class="nav-item" onclick="switchPage('tactical')"><span class="nav-icon">&#128506;</span><span class="nav-label">TAKT&#304;K</span></div>
        <div class="nav-item" onclick="switchPage('entities')"><span class="nav-icon">&#127919;</span><span class="nav-label">VARLIKLAR</span></div>
        <div class="nav-item" onclick="switchPage('events')"><span class="nav-icon">&#9889;</span><span class="nav-label">OLAYLAR</span></div>
        <div class="nav-item" onclick="switchPage('modules')"><span class="nav-icon">&#127963;</span><span class="nav-label">MOD&#220;LLER</span></div>
      </div>
    </div>
    <div class="content">
      <div class="page active" id="page-overview">
        <div class="kpi-row">
          <div class="kpi-card"><div class="kpi-val" id="kpiModules" style="color:var(--green)">0/0</div><div class="kpi-lbl">Aktif Mod&#252;ller</div></div>
          <div class="kpi-card"><div class="kpi-val" id="kpiEntities" style="color:var(--cyan)">0</div><div class="kpi-lbl">Toplam Varl&#305;k</div></div>
          <div class="kpi-card"><div class="kpi-val" id="kpiFriendly" style="color:var(--blue)">0</div><div class="kpi-lbl">Dost Kuvvetler</div></div>
          <div class="kpi-card"><div class="kpi-val" id="kpiEnemy" style="color:var(--red)">0</div><div class="kpi-lbl">D&#252;&#351;man Tespiti</div></div>
          <div class="kpi-card"><div class="kpi-val" id="kpiEvents" style="color:var(--yellow)">0</div><div class="kpi-lbl">Aktif Olay</div></div>
          <div class="kpi-card"><div class="kpi-val" id="kpiLatency" style="color:var(--purple)">0</div><div class="kpi-lbl">Ort. Gecikme</div><div class="kpi-sub">ms</div></div>
        </div>
        <div class="overview-grid">
          <div class="panel-box" style="min-height:300px">
            <div class="panel-header"><span class="panel-title">Ger&#231;ek Zamanl&#305; Harita</span></div>
            <div style="flex:1;position:relative;cursor:pointer" onclick="switchPage('tactical')"><canvas id="miniMapCanvas"></canvas></div>
          </div>
          <div class="panel-box">
            <div class="panel-header"><span class="panel-title">Tehdit &#214;zeti</span></div>
            <div class="panel-body" id="threatSummary"></div>
          </div>
        </div>
        <div class="overview-grid2">
          <div class="panel-box" style="max-height:300px">
            <div class="panel-header"><span class="panel-title">Son Olaylar</span><span class="panel-badge" id="recentEvtBadge">0</span></div>
            <div class="panel-body" id="recentEvents"></div>
          </div>
          <div class="panel-box" style="max-height:300px">
            <div class="panel-header"><span class="panel-title">Mod&#252;l Durumu</span><span class="panel-badge" id="modStatusBadge">0 aktif</span></div>
            <div class="panel-body" id="moduleStatusGrid"></div>
          </div>
        </div>
      </div>
      <div class="page page-fullscreen" id="page-tactical">
        <div class="tactical-wrap">
          <canvas id="mapCanvas"></canvas>
          <div class="float-panel float-controls" id="mapControls">
            <button class="map-btn active" data-mode="tactical" onclick="setMapMode('tactical')">&#11047; Taktik</button>
            <button class="map-btn" data-mode="map" onclick="setMapMode('map')">&#128506; Harita</button>
            <button class="map-btn" data-mode="satellite" onclick="setMapMode('satellite')">&#128752; Uydu</button>
          </div>
          <div class="float-panel float-layers" id="floatLayers"></div>
          <div class="float-panel float-entities" id="floatEntities"></div>
          <div class="float-panel float-detail" id="entityDetail">
            <div class="ed-header">
              <button class="ed-close" onclick="closeEntityDetail()">&#10005;</button>
              <span class="ed-icon" id="edIcon">&#9635;</span>
              <div class="ed-title"><h3 id="edName">&#8212;</h3><div class="ed-sub" id="edSub">&#8212;</div></div>
              <span id="edFaction" style="padding:3px 10px;border-radius:10px;font-size:10px;font-weight:700">&#8212;</span>
            </div>
            <div class="ed-body" id="edBody"></div>
          </div>
          <div class="overlay-info" id="mapInfo">Harita y&#252;kleniyor...</div>
        </div>
      </div>
      <div class="page" id="page-entities">
        <div class="table-toolbar">
          <input class="table-search" id="entSearchInput" type="text" placeholder="Varl&#305;k ara... (isim, t&#252;r, taraf)">
          <div class="filter-group" id="factionFilters">
            <button class="filter-btn active" onclick="setFactionFilter('all')">T&#252;m&#252;</button>
            <button class="filter-btn" onclick="setFactionFilter('friendly')">Dost</button>
            <button class="filter-btn" onclick="setFactionFilter('enemy')">D&#252;&#351;man</button>
            <button class="filter-btn" onclick="setFactionFilter('neutral')">Tarafs&#305;z</button>
          </div>
          <div class="filter-group" id="statusFilters">
            <button class="filter-btn active" onclick="setStatusFilter('all')">T&#252;m&#252;</button>
            <button class="filter-btn" onclick="setStatusFilter('active')">Aktif</button>
            <button class="filter-btn" onclick="setStatusFilter('damaged')">Hasarl&#305;</button>
            <button class="filter-btn" onclick="setStatusFilter('destroyed')">&#304;mha</button>
          </div>
          <span style="font-size:10px;color:var(--dim)" id="entFilterStatus">0 varl&#305;k</span>
        </div>
        <div style="flex:1;overflow-y:auto">
          <table class="data-table"><thead id="entTableHead"></thead><tbody id="entTableBody"></tbody></table>
        </div>
      </div>
      <div class="page" id="page-events">
        <div class="events-toolbar">
          <div class="sev-pills" id="sevPills">
            <button class="sev-pill active" onclick="setSevFilter('all')">T&#252;m&#252;</button>
            <button class="sev-pill" onclick="setSevFilter('info')">Bilgi</button>
            <button class="sev-pill" onclick="setSevFilter('warning')">Uyar&#305;</button>
            <button class="sev-pill" onclick="setSevFilter('critical')">Kritik</button>
          </div>
          <div class="sev-counts" id="sevCounts"></div>
        </div>
        <div style="overflow-y:auto;flex:1" id="eventsTimeline"></div>
      </div>
      <div class="page" id="page-modules">
        <div class="module-grid" id="moduleGrid"></div>
      </div>

    </div>
  </div>
</div>
<div class="map-tooltip" id="mapTooltip"></div>
<script>
var API='';
var modules={},entities=[],events=[],layers=[];
var currentPage='overview',selectedEntity=null,hoveredEntity=null;
var mapCanvas,mapCtx,miniMapCanvas,miniMapCtx,mapW=0,mapH=0,miniW=0,miniH=0;
var centerLat=39.925,centerLon=32.837,zoom=1;
var dragging=false,dragStartX=0,dragStartY=0,dragStartLat=0,dragStartLon=0,dragMoved=false;
var latencyHistory=[],moduleLatHistory={};
var mapMode='tactical',tileCache={},autoFitted=false;
var entSearchQuery='',entFactionFilter='all',entStatusFilter='all';
var entSortCol='id',entSortDir='asc';
var evtSevFilter='all';
var TILE_SIZE=256;
var TILE_SERVERS={map:'https://tile.openstreetmap.org/{z}/{x}/{y}.png',satellite:'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}'};
var MODULE_ICONS={butun:'\u{1F3DB}\u{FE0F}',bag:'\u{1F4E1}',kutuk:'\u{1F5C4}\u{FE0F}',dirlik:'\u{1F3E5}',bagdas:'\u{1F504}',akim:'\u{1F30A}',yurt:'\u{1F3D5}\u{FE0F}',im:'\u{1F510}',yayin:'\u{1F4E1}',durak:'\u23F8\u{FE0F}',tutum:'\u{1F50B}',tani:'\u{1F3AF}',koz:'\u{1F525}'};
var MODULE_NAMES={butun:'B\u00dcT\u00dcN',bag:'BA\u011e',kutuk:'K\u00dcT\u00dcK',dirlik:'D\u0130RL\u0130K',bagdas:'BA\u011eDA\u015e',akim:'AKIM',yurt:'YURT',im:'\u0130M',yayin:'YAYIN',durak:'DURAK',tutum:'TUTUM',tani:'TANI',koz:'K\u00d6Z'};
var MODULE_DESC={butun:'Orkestrasyon & API Gateway',bag:'A\u011f \u0130leti\u015fim Protokol\u00fc',kutuk:'Da\u011f\u0131t\u0131k Log Altyap\u0131s\u0131',dirlik:'Sa\u011fl\u0131k & Hata Y\u00f6netimi',bagdas:'Protokol Uyumland\u0131rma',akim:'Veri Ak\u0131\u015f Y\u00f6neticisi',yurt:'Da\u011f\u0131t\u0131k Konum Y\u00f6netimi',im:'Dijital Kimlik & \u015eifreleme',yayin:'\u015eifreli Veri Yay\u0131m\u0131',durak:'Paket QoS & Kuyruk',tutum:'Batarya & Kaynak \u0130zleme',tani:'Nesne Tespit & S\u0131n\u0131fland\u0131rma',koz:'Termal \u0130z & Motor Analizi'};
var MODULE_PORTS={butun:8000,bag:9091,kutuk:9093,dirlik:9095,bagdas:9097,akim:9099,yurt:9101,im:9105,yayin:9107,durak:9109,tutum:9111,tani:9113,koz:9115};
var ENT_ICONS={drone:'\u2708',uav:'\u2708',aircraft:'\u2708',helicopter:'\u{1F681}',fighter_jet:'\u2708',tank:'\u25A3',apc:'\u25A3',vehicle:'\u25A3',humvee:'\u25A3',truck:'\u25A3',infantry:'\u263A',soldier:'\u263A',sniper:'\u263A',civilian:'\u{1F464}',radar:'\u25CE',sam_launcher:'\u25C6',artillery:'\u25C6',command_post:'\u2B22',sensor:'\u25CE',base:'\u2B21',ship:'\u26F5'};
var TYPE_LABELS={drone:'\u0130HA',uav:'\u0130HA',aircraft:'U\u00e7ak',helicopter:'Helikopter',fighter_jet:'Sava\u015f U\u00e7a\u011f\u0131',tank:'Tank',apc:'Z\u0131rhl\u0131 Ara\u00e7',vehicle:'Ara\u00e7',humvee:'Taktik Ara\u00e7',truck:'Kamyon',infantry:'Piyade',soldier:'Asker',sniper:'Keskin Ni\u015fanc\u0131',civilian:'Sivil',radar:'Radar',sam_launcher:'SAM Rampas\u0131',artillery:'Top\u00e7u',command_post:'Komuta Merkezi',sensor:'Sens\u00f6r',base:'\u00dcs',ship:'Gemi'};
var FACTION_LABELS={friendly:'DOST',enemy:'D\u00dc\u015eMAN',neutral:'TARAFSIZ',unknown:'B\u0130L\u0130NMEYEN'};
var FACTION_COLORS={friendly:'#2d7fff',enemy:'#ff4d6a',neutral:'#ffc233',unknown:'#5a6d8a'};
var INTEL_LABELS={0:'Tespit Yok',1:'Minimum',2:'Temel',3:'Detayl\u0131',4:'Tam \u0130stihbarat'};
var INTEL_COLORS={0:'#555',1:'#ff4d6a',2:'#f5a623',3:'#66d9ef',4:'#00e5a0'};
var STATUS_LABELS={active:'Aktif',damaged:'Hasarl\u0131',destroyed:'\u0130mha',retreating:'Geri \u00c7ekilme',idle:'Beklemede',moving:'Hareket',engaging:'\u00c7at\u0131\u015fma',defending:'Savunma'};
var EVT_ICONS={enemy_detected:'\u{1F6A8}',entity_destroyed:'\u{1F4A5}',unit_destroyed:'\u{1F4A5}',health_critical:'\u2764\u{FE0F}\u200D\u{1F525}',entity_damaged:'\u26A0\u{FE0F}',heavy_damage:'\u26A0\u{FE0F}',friendly_detected:'\u{1F91D}',low_fuel_alert:'\u26FD',enemy_maneuver:'\u{1F5FA}\u{FE0F}',c2_alert:'\u{1F4E2}',module_up:'\u2705',module_down:'\u274C',system_ready:'\u{1F680}',critical:'\u{1F534}',warning:'\u{1F7E1}',info:'\u{1F535}'};
function applyIntel(raw){var il=raw.intel_level!==undefined?raw.intel_level:4;if(il>=4)return raw;var e={id:raw.id,lat:raw.lat,lon:raw.lon,last_update:raw.last_update,intel_level:il,faction:il>=1?raw.faction:'unknown'};if(il>=2){e.type=raw.type;e.heading=raw.heading;e.speed=raw.speed}if(il>=3){e.status=raw.status;e.health=raw.health;e.altitude=raw.altitude;e.threat_level=raw.threat_level}if(il>=4){e.fuel=raw.fuel;e.callsign=raw.callsign;e.unit=raw.unit;e.weapon=raw.weapon;e.ammo=raw.ammo;e.sensor_range=raw.sensor_range;e.comm_freq=raw.comm_freq;e.iff_code=raw.iff_code;e.mission=raw.mission}return e}
function init(){
  mapCanvas=document.getElementById('mapCanvas');mapCtx=mapCanvas.getContext('2d');
  miniMapCanvas=document.getElementById('miniMapCanvas');miniMapCtx=miniMapCanvas.getContext('2d');
  resizeCanvases();window.addEventListener('resize',resizeCanvases);
  setupMapInteraction();
  document.getElementById('entSearchInput').addEventListener('input',function(e){entSearchQuery=e.target.value.toLowerCase();renderEntityTable()});
  connectSSE();fetchAll();setInterval(fetchAll,2000);setInterval(updateClock,1000);requestAnimationFrame(renderLoop);
}
function resizeCanvases(){
  var dpr=window.devicePixelRatio||1;
  if(mapCanvas&&mapCanvas.parentElement){var r=mapCanvas.parentElement.getBoundingClientRect();mapW=r.width;mapH=r.height;mapCanvas.width=Math.round(mapW*dpr);mapCanvas.height=Math.round(mapH*dpr);mapCtx.setTransform(dpr,0,0,dpr,0,0)}
  if(miniMapCanvas&&miniMapCanvas.parentElement){var r2=miniMapCanvas.parentElement.getBoundingClientRect();if(r2.width>0){miniW=r2.width;miniH=r2.height;miniMapCanvas.width=Math.round(miniW);miniMapCanvas.height=Math.round(miniH)}}
}
function switchPage(page){
  currentPage=page;
  var pages=['overview','tactical','entities','events','modules'];
  var labels=['GENEL BAKI\u015e','TAKT\u0130K','VARLIKLAR','OLAYLAR','MOD\u00dcLLER'];
  var navItems=document.querySelectorAll('.nav-item');
  for(var i=0;i<navItems.length;i++){navItems[i].className='nav-item'+(pages[i]===page?' active':'')}
  for(var i=0;i<pages.length;i++){
    var el=document.getElementById('page-'+pages[i]);
    if(pages[i]==='tactical'){el.className='page page-fullscreen'+(pages[i]===page?' active':'')}
    else{el.className='page'+(pages[i]===page?' active':'')}
  }
  document.getElementById('breadcrumb').textContent=labels[pages.indexOf(page)];
  setTimeout(resizeCanvases,50);
  if(page==='overview')renderOverviewPage();
  if(page==='tactical'){renderFloatLayers();renderFloatEntities()}
  if(page==='entities')renderEntityTable();
  if(page==='events')renderEventsTimeline();
  if(page==='modules')renderModuleHealth();
}
function updateClock(){document.getElementById('clockEl').textContent=new Date().toLocaleTimeString('tr-TR',{hour:'2-digit',minute:'2-digit',second:'2-digit'})}
function connectSSE(){var es=new EventSource(API+'/stream');es.onopen=function(){document.getElementById('connBadge').className='conn-badge ok';document.getElementById('connText').textContent='Ba\u011fl\u0131'};es.onerror=function(){document.getElementById('connBadge').className='conn-badge err';document.getElementById('connText').textContent='Kopuk'};es.onmessage=function(e){try{var d=JSON.parse(e.data);if(d.type==='event')addEvent(d.event);else if(d.type==='update')processUpdate(d);else if(d.modules)processUpdate(d)}catch(err){}}}
function processUpdate(d){if(d.modules){modules=d.modules;updateModuleLatencyHistory()}if(d.stats){var s=d.stats;var avgLat=s.health_checks>0?Math.round(s.total_latency_ms/s.health_checks):0;latencyHistory.push(avgLat);if(latencyHistory.length>60)latencyHistory.shift()}renderTopBarStats()}
function updateModuleLatencyHistory(){for(var id in modules){if(!moduleLatHistory[id])moduleLatHistory[id]=[];moduleLatHistory[id].push(modules[id].latency_ms||0);if(moduleLatHistory[id].length>30)moduleLatHistory[id].shift()}}
function fetchAll(){
  Promise.all([
    fetch(API+'/modules').then(function(r){return r.json()}).catch(function(){return {}}),
    fetch(API+'/entities').then(function(r){return r.json()}).catch(function(){return []}),
    fetch(API+'/events?n=200').then(function(r){return r.json()}).catch(function(){return []}),
    fetch(API+'/layers').then(function(r){return r.json()}).catch(function(){return []})
  ]).then(function(results){
    modules=results[0];entities=(results[1]||[]).map(applyIntel);events=results[2]||[];layers=results[3]||[];
    updateModuleLatencyHistory();
    if(!autoFitted&&entities.length>0)autoFitted=false;
    renderTopBarStats();
    if(currentPage==='overview')renderOverviewPage();
    if(currentPage==='tactical'){renderFloatLayers();renderFloatEntities()}
    if(currentPage==='entities')renderEntityTable();
    if(currentPage==='events')renderEventsTimeline();
    if(currentPage==='modules')renderModuleHealth();
    if(selectedEntity){var se=null;for(var i=0;i<entities.length;i++){if(entities[i].id===selectedEntity){se=entities[i];break}}if(se)renderEntityDetail(se)}
  }).catch(function(){document.getElementById('mapInfo').textContent='Ba\u011flant\u0131 hatas\u0131'})
}
function addEvent(evt){events.unshift(evt);if(events.length>200)events.pop();document.getElementById('evtCount').textContent=events.length;if(currentPage==='overview')renderRecentEvents();if(currentPage==='events')renderEventsTimeline()}
function renderTopBarStats(){var alive=0,total=0;for(var id in modules){total++;if(modules[id].alive)alive++}document.getElementById('modAlive').textContent=alive;document.getElementById('modTotal').textContent=total;document.getElementById('entCount').textContent=entities.length;document.getElementById('evtCount').textContent=events.length}
function renderOverviewPage(){renderKPICards();renderThreatSummary();renderRecentEvents();renderModuleStatusGrid()}
function renderKPICards(){
  var alive=0,total=0;for(var id in modules){total++;if(modules[id].alive)alive++}
  var fc={friendly:0,enemy:0,neutral:0,unknown:0};for(var i=0;i<entities.length;i++){var f=entities[i].faction||'unknown';fc[f]=(fc[f]||0)+1}
  var avgLat=latencyHistory.length>0?latencyHistory[latencyHistory.length-1]:0;
  document.getElementById('kpiModules').textContent=alive+'/'+total;
  document.getElementById('kpiEntities').textContent=entities.length;
  document.getElementById('kpiFriendly').textContent=fc.friendly;
  document.getElementById('kpiEnemy').textContent=fc.enemy;
  document.getElementById('kpiEvents').textContent=events.length;
  document.getElementById('kpiLatency').textContent=avgLat;
}
function renderThreatSummary(){
  var el=document.getElementById('threatSummary');
  var fc={friendly:0,enemy:0,neutral:0,unknown:0};for(var i=0;i<entities.length;i++){var f=entities[i].faction||'unknown';fc[f]=(fc[f]||0)+1}
  var total=entities.length||1;
  var html='<div style="margin-bottom:12px"><div style="font-size:10px;color:var(--dim);font-weight:700;margin-bottom:6px;text-transform:uppercase;letter-spacing:1px">Taraf Da\u011f\u0131l\u0131m\u0131</div>';
  html+='<div class="faction-bar"><div style="width:'+(fc.friendly/total*100)+'%;background:var(--friendly)"></div><div style="width:'+(fc.enemy/total*100)+'%;background:var(--enemy)"></div><div style="width:'+(fc.neutral/total*100)+'%;background:var(--neutral)"></div><div style="width:'+(fc.unknown/total*100)+'%;background:var(--unknown)"></div></div>';
  html+='<div class="faction-legend"><span><span class="faction-dot" style="background:var(--friendly)"></span>Dost '+fc.friendly+'</span><span><span class="faction-dot" style="background:var(--enemy)"></span>D\u00fc\u015fman '+fc.enemy+'</span><span><span class="faction-dot" style="background:var(--neutral)"></span>Tarafs\u0131z '+fc.neutral+'</span></div></div>';
  var enemies=[];for(var i=0;i<entities.length;i++){if(entities[i].faction==='enemy')enemies.push(entities[i])}
  if(enemies.length>0){
    html+='<div style="font-size:10px;color:var(--red);font-weight:700;margin:8px 0 6px;text-transform:uppercase;letter-spacing:1px">\u26A0 D\u00fc\u015fman Varl\u0131klar\u0131</div>';
    for(var i=0;i<Math.min(enemies.length,5);i++){var e=enemies[i];html+='<div style="background:rgba(255,77,106,.06);border:1px solid rgba(255,77,106,.15);border-radius:6px;padding:8px 10px;margin-bottom:4px;font-size:10px"><div style="font-weight:700;color:var(--red)">'+(e.id||'?')+'</div><div style="color:var(--dim)">'+(TYPE_LABELS[e.type]||e.type||'?')+' | '+(e.lat?e.lat.toFixed(4):'?')+'N, '+(e.lon?e.lon.toFixed(4):'?')+'E</div></div>'}
  }else{html+='<div style="text-align:center;padding:20px;color:var(--green);font-size:11px">\u2713 D\u00fc\u015fman tespiti yok</div>'}
  el.innerHTML=html;
}
function renderRecentEvents(){
  var el=document.getElementById('recentEvents');document.getElementById('recentEvtBadge').textContent=events.length;
  var tactical=events.filter(function(e){return e.type!=='module_up'&&e.type!=='entities_updated'});
  var show=tactical.length>0?tactical:events;
  var html='';for(var i=0;i<Math.min(show.length,10);i++){var evt=show[i];var sev=evt.severity||'info';var time=new Date(evt.timestamp).toLocaleTimeString('tr-TR');
    var icon=EVT_ICONS[evt.type]||EVT_ICONS[sev]||'\u2022';
    html+='<div style="display:flex;align-items:center;gap:8px;padding:6px 0;border-bottom:1px solid rgba(28,42,69,.3)"><span style="font-size:14px;flex-shrink:0">'+icon+'</span><span class="sev-dot '+sev+'"></span><span style="flex:1;font-size:11px;font-weight:500">'+(evt.title||evt.type||'-')+'</span><span style="font-size:9px;color:var(--muted);font-variant-numeric:tabular-nums;flex-shrink:0">'+time+'</span></div>'}
  if(!show.length)html='<div style="color:var(--muted);font-size:11px;text-align:center;padding:20px">Olay bekleniyor...</div>';
  el.innerHTML=html;
}
function renderModuleStatusGrid(){
  var el=document.getElementById('moduleStatusGrid');var alive=0,total=0;
  var html='<div class="mod-grid-sm">';
  for(var id in modules){var m=modules[id];total++;if(m.alive)alive++;var icon=MODULE_ICONS[id]||'\u{1F4E6}';var name=MODULE_NAMES[id]||id.toUpperCase();
    html+='<div class="mod-grid-sm-item"><div style="font-size:16px">'+icon+'</div><div style="font-size:9px;font-weight:700;margin-top:2px">'+name+'</div><div style="margin-top:4px"><span class="mod-card-status '+(m.alive?'alive':'dead')+'" style="display:inline-block"></span></div></div>'}
  html+='</div>';document.getElementById('modStatusBadge').textContent=alive+' aktif';el.innerHTML=html;
}
function renderFloatLayers(){
  var el=document.getElementById('floatLayers');
  var html='<div style="font-size:9px;font-weight:700;color:var(--dim);text-transform:uppercase;letter-spacing:1px;margin-bottom:6px">Katmanlar</div>';
  for(var i=0;i<layers.length;i++){var l=layers[i];var on=l.visible;
    html+='<div class="layer-row" onclick="toggleLayer(\''+l.id+'\')" style="opacity:'+(on?'1':'0.4')+'"><div class="layer-check '+(on?'on':'')+'">'+(on?'\u2713':'')+'</div><span style="flex:1">'+( l.name||l.id)+'</span></div>'}
  el.innerHTML=html;
}
function renderFloatEntities(){
  var el=document.getElementById('floatEntities');
  var html='<div style="font-size:9px;font-weight:700;color:var(--dim);text-transform:uppercase;letter-spacing:1px;margin-bottom:6px">Varl\u0131klar ('+entities.length+')</div>';
  var sorted=entities.slice().sort(function(a,b){var fo={enemy:0,friendly:1,neutral:2,unknown:3};return(fo[a.faction]||3)-(fo[b.faction]||3)});
  for(var i=0;i<Math.min(sorted.length,50);i++){var e=sorted[i];var fc=FACTION_COLORS[e.faction]||'#5a6d8a';var icon=ENT_ICONS[e.type]||'\u25C6';var sel=e.id===selectedEntity?' selected':'';
    html+='<div class="mini-ent'+sel+'" onclick="selectEntity(\''+e.id+'\')"><span style="color:'+fc+'">'+icon+'</span><span style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">'+(e.id||'?')+'</span></div>'}
  el.innerHTML=html;
}
function renderEntityTable(){renderEntTableHead();renderEntTableBody()}
function renderEntTableHead(){
  var cols=[{k:'id',l:'ID'},{k:'type',l:'T\u00fcr'},{k:'faction',l:'Taraf'},{k:'status',l:'Durum'},{k:'health',l:'Sa\u011fl\u0131k%'},{k:'lat',l:'Enlem'},{k:'lon',l:'Boylam'},{k:'speed',l:'H\u0131z'},{k:'heading',l:'Rota'},{k:'altitude',l:'\u0130rtifa'},{k:'intel_level',l:'\u0130stihbarat'},{k:'last_update',l:'Son G\u00fcncelleme'}];
  var html='<tr>';for(var i=0;i<cols.length;i++){var c=cols[i];var arrow='';if(entSortCol===c.k)arrow=entSortDir==='asc'?' \u25B2':' \u25BC';
    html+='<th onclick="sortEntityTable(\''+c.k+'\')">'+c.l+'<span class="sort-arrow">'+arrow+'</span></th>'}
  html+='</tr>';document.getElementById('entTableHead').innerHTML=html;
}
function renderEntTableBody(){
  var filtered=filterEntitiesForTable();var sorted=sortEntitiesArray(filtered);
  document.getElementById('entFilterStatus').textContent=sorted.length+' / '+entities.length+' varl\u0131k';
  var html='';for(var i=0;i<Math.min(sorted.length,500);i++){var e=sorted[i];var fColor=FACTION_COLORS[e.faction]||'#5a6d8a';
    var hp=e.health!==undefined?e.health:-1;var hpColor=hp>70?'var(--green)':hp>30?'var(--yellow)':'var(--red)';
    var sel=e.id===selectedEntity?' selected':'';var lastUp='';
    if(e.last_update){var lu=typeof e.last_update==='number'?new Date(e.last_update>1e12?e.last_update:e.last_update*1000):new Date(e.last_update);lastUp=lu.toLocaleTimeString('tr-TR')}
    html+='<tr class="'+sel+'" onclick="selectEntityAndGo(\''+e.id+'\')" style="border-left:3px solid '+fColor+'">';
    html+='<td style="font-weight:600">'+(e.id||'-')+'</td>';
    html+='<td>'+(TYPE_LABELS[e.type]||e.type||'-')+'</td>';
    html+='<td style="color:'+fColor+'">'+(FACTION_LABELS[e.faction]||e.faction||'-')+'</td>';
    html+='<td>'+(STATUS_LABELS[e.status]||e.status||'-')+'</td>';
    html+='<td>';if(hp>=0){html+='<div class="hp-mini"><span class="hp-mini-fill" style="width:'+Math.max(0,hp)+'%;background:'+hpColor+'"></span></div> '+hp.toFixed(0)+'%'}else{html+='-'}html+='</td>';
    html+='<td>'+(e.lat?e.lat.toFixed(5):'-')+'</td>';
    html+='<td>'+(e.lon?e.lon.toFixed(5):'-')+'</td>';
    html+='<td>'+(e.speed!==undefined?e.speed.toFixed(1):'-')+'</td>';
    html+='<td>'+(e.heading!==undefined?e.heading.toFixed(0)+'\u00b0':'-')+'</td>';
    html+='<td>'+(e.altitude?e.altitude.toFixed(0)+'m':'-')+'</td>';
    html+='<td style="color:'+(INTEL_COLORS[e.intel_level!==undefined?e.intel_level:4]||'#555')+'">'+(INTEL_LABELS[e.intel_level!==undefined?e.intel_level:4]||'-')+'</td>';
    html+='<td style="font-variant-numeric:tabular-nums">'+lastUp+'</td></tr>'}
  if(sorted.length===0)html='<tr><td colspan="12" style="text-align:center;padding:20px;color:var(--muted)">Varl\u0131k bulunamad\u0131</td></tr>';
  document.getElementById('entTableBody').innerHTML=html;
}
function filterEntitiesForTable(){return entities.filter(function(e){
  if(entSearchQuery){var s=entSearchQuery;var m=(e.id||'').toLowerCase().indexOf(s)>=0||(e.type||'').toLowerCase().indexOf(s)>=0||(e.faction||'').toLowerCase().indexOf(s)>=0||(TYPE_LABELS[e.type]||'').toLowerCase().indexOf(s)>=0||(FACTION_LABELS[e.faction]||'').toLowerCase().indexOf(s)>=0;if(!m)return false}
  if(entFactionFilter!=='all'&&e.faction!==entFactionFilter)return false;
  if(entStatusFilter!=='all'){if(entStatusFilter==='destroyed'&&e.status!=='destroyed'&&!(e.health!==undefined&&e.health<=0))return false;if(entStatusFilter==='damaged'&&e.status!=='damaged'&&e.status!=='retreating')return false;if(entStatusFilter==='active'&&(e.status==='destroyed'||(e.health!==undefined&&e.health<=0)))return false}
  return true})}
function sortEntitiesArray(arr){return arr.slice().sort(function(a,b){var av=a[entSortCol],bv=b[entSortCol];if(av===undefined||av===null)av='';if(bv===undefined||bv===null)bv='';var cmp=0;if(typeof av==='number'&&typeof bv==='number')cmp=av-bv;else cmp=String(av).localeCompare(String(bv));return entSortDir==='asc'?cmp:-cmp})}
function sortEntityTable(col){if(entSortCol===col)entSortDir=entSortDir==='asc'?'desc':'asc';else{entSortCol=col;entSortDir='asc'}renderEntityTable()}
function setFactionFilter(f){entFactionFilter=f;var btns=document.getElementById('factionFilters').children;var vals=['all','friendly','enemy','neutral'];for(var i=0;i<btns.length;i++)btns[i].className='filter-btn'+(vals[i]===f?' active':'');renderEntityTable()}
function setStatusFilter(s){entStatusFilter=s;var btns=document.getElementById('statusFilters').children;var vals=['all','active','damaged','destroyed'];for(var i=0;i<btns.length;i++)btns[i].className='filter-btn'+(vals[i]===s?' active':'');renderEntityTable()}
function selectEntityAndGo(id){selectEntity(id);switchPage('tactical')}
function renderEventsTimeline(){
  var sevCounts={info:0,warning:0,critical:0,success:0};for(var i=0;i<events.length;i++){var sev=events[i].severity||'info';sevCounts[sev]=(sevCounts[sev]||0)+1}
  document.getElementById('sevCounts').innerHTML='<span style="color:var(--cyan)">Bilgi: '+(sevCounts.info||0)+'</span><span style="color:var(--yellow)">Uyar\u0131: '+(sevCounts.warning||0)+'</span><span style="color:var(--red)">Kritik: '+(sevCounts.critical||0)+'</span>';
  var filtered=events.filter(function(e){if(evtSevFilter==='all')return true;return(e.severity||'info')===evtSevFilter});
  var html='';var prevDate='';for(var i=0;i<Math.min(filtered.length,100);i++){var evt=filtered[i];var sev=evt.severity||'info';var dt=new Date(evt.timestamp);var dateStr=dt.toLocaleDateString('tr-TR');var timeStr=dt.toLocaleTimeString('tr-TR');
    if(dateStr!==prevDate){html+='<div style="font-size:10px;font-weight:700;color:var(--dim);text-transform:uppercase;letter-spacing:1px;padding:12px 0 6px;border-bottom:1px solid rgba(28,42,69,.3)">'+dateStr+'</div>';prevDate=dateStr}
    var icon=EVT_ICONS[evt.type]||EVT_ICONS[sev]||'\u2022';
    var sevLabel=sev==='critical'?'KR\u0130T\u0130K':sev==='warning'?'UYARI':'B\u0130LG\u0130';
    html+='<div class="event-card"><div class="event-card-header"><span style="font-size:16px;flex-shrink:0">'+icon+'</span><span class="sev-dot '+sev+'"></span><span class="event-title">'+(evt.title||evt.type||'-')+'</span><span class="event-sev-label sev-label-'+sev+'">'+sevLabel+'</span><span class="event-time">'+timeStr+'</span></div>';
    html+='<span class="event-source">'+(evt.source||'S\u0130STEM')+'</span>';
    if(evt.detail||evt.details)html+='<div class="event-detail">'+(evt.detail||evt.details)+'</div>';
    if(evt.lat&&evt.lon)html+='<div class="event-detail" style="color:var(--dim)">Konum: '+evt.lat.toFixed(4)+'N, '+evt.lon.toFixed(4)+'E</div>';
    html+='</div>'}
  if(filtered.length===0)html='<div style="text-align:center;padding:40px;color:var(--muted)">Olay bulunamad\u0131</div>';
  document.getElementById('eventsTimeline').innerHTML=html;
}
function setSevFilter(s){evtSevFilter=s;var btns=document.querySelectorAll('.sev-pill');var vals=['all','info','warning','critical'];for(var i=0;i<btns.length;i++)btns[i].className='sev-pill'+(vals[i]===s?' active':'');renderEventsTimeline()}
function renderModuleHealth(){
  var el=document.getElementById('moduleGrid');var html='';
  for(var id in modules){var m=modules[id];var icon=MODULE_ICONS[id]||'\u{1F4E6}';var name=MODULE_NAMES[id]||id.toUpperCase();var desc=MODULE_DESC[id]||'';var port=MODULE_PORTS[id]||'-';
    var latMs=m.latency_ms?m.latency_ms.toFixed(0):'-';var lastCheck='';
    if(m.last_check){var lc=typeof m.last_check==='number'?new Date(m.last_check>1e12?m.last_check:m.last_check*1000):new Date(m.last_check);lastCheck=lc.toLocaleTimeString('tr-TR')}
    html+='<div class="mod-card"><div class="mod-card-header"><span class="mod-card-icon">'+icon+'</span><div class="mod-card-info"><div class="mod-card-name">'+name+'</div><div class="mod-card-desc">'+desc+'</div></div><span class="mod-card-status '+(m.alive?'alive':'dead')+'"></span></div>';
    html+='<div class="mod-card-body"><div class="mod-card-stats">';
    html+='<div class="mod-stat"><div class="val" style="color:'+(m.alive?'var(--green)':'var(--red)')+'">'+(m.alive?'AKT\u0130F':'KAPALI')+'</div><div class="lbl">Durum</div></div>';
    html+='<div class="mod-stat"><div class="val">'+latMs+'</div><div class="lbl">Gecikme (ms)</div></div>';
    html+='<div class="mod-stat"><div class="val">'+port+'</div><div class="lbl">Port</div></div>';
    html+='<div class="mod-stat"><div class="val" style="font-size:10px">'+(lastCheck||'-')+'</div><div class="lbl">Son Kontrol</div></div>';
    html+='</div><canvas class="spark-chart" id="spark-'+id+'" width="200" height="40"></canvas></div></div>'}
  el.innerHTML=html;for(var id in modules)renderSparkChart(id);
}
function renderSparkChart(modId){
  var canvas=document.getElementById('spark-'+modId);if(!canvas)return;var ctx2=canvas.getContext('2d');
  var data=moduleLatHistory[modId]||[];var W=canvas.width,H=canvas.height;ctx2.clearRect(0,0,W,H);if(data.length<2)return;
  var mx=Math.max.apply(null,data)||1;ctx2.beginPath();
  for(var i=0;i<data.length;i++){var x=i/(data.length-1)*W;var y=H-(data[i]/mx)*(H-4)-2;if(i===0)ctx2.moveTo(x,y);else ctx2.lineTo(x,y)}
  ctx2.strokeStyle=modules[modId]&&modules[modId].alive?'#00e68a':'#ff4d6a';ctx2.lineWidth=1.5;ctx2.stroke();
  ctx2.lineTo(W,H);ctx2.lineTo(0,H);ctx2.closePath();
  var grad=ctx2.createLinearGradient(0,0,0,H);var clr=modules[modId]&&modules[modId].alive?'0,230,138':'255,77,106';
  grad.addColorStop(0,'rgba('+clr+',0.2)');grad.addColorStop(1,'rgba('+clr+',0)');ctx2.fillStyle=grad;ctx2.fill();
}
function setupMapInteraction(){
  mapCanvas.addEventListener('wheel',function(e){e.preventDefault();var rect=mapCanvas.getBoundingClientRect();var mx=e.clientX-rect.left,my=e.clientY-rect.top;var before=screenToLatLon(mx,my,mapW,mapH);var factor=e.deltaY>0?0.85:1.18;zoom=Math.max(0.05,Math.min(10,zoom*factor));var after=screenToLatLon(mx,my,mapW,mapH);centerLat+=before.lat-after.lat;centerLon+=before.lon-after.lon;autoFitted=true},{passive:false});
  mapCanvas.addEventListener('mousedown',function(e){dragging=true;dragMoved=false;dragStartX=e.clientX;dragStartY=e.clientY;dragStartLat=centerLat;dragStartLon=centerLon;mapCanvas.style.cursor='grabbing'});
  mapCanvas.addEventListener('mousemove',function(e){
    if(!dragging){updateHover(e);return}
    if(!dragMoved&&(Math.abs(e.clientX-dragStartX)>3||Math.abs(e.clientY-dragStartY)>3))dragMoved=true;
    if(!dragMoved)return;
    var dx=e.clientX-dragStartX,dy=e.clientY-dragStartY;
    if(mapMode==='tactical'){var p=5000*zoom;centerLon=dragStartLon-dx/p;centerLat=dragStartLat+dy/p}
    else{var fz=getOsmZoom(),iz=Math.max(2,Math.min(18,Math.round(fz))),ws=TILE_SIZE*(1<<iz),sf=Math.pow(2,fz-iz);
      centerLon=dragStartLon-(dx/sf)/ws*360;var origMY=mercY(dragStartLat)*ws,newMY=origMY+(dy/sf),my01=newMY/ws;
      centerLat=Math.atan(Math.sinh(Math.PI*(1-2*my01)))*180/Math.PI;centerLat=Math.max(-85,Math.min(85,centerLat))}
    autoFitted=true});
  mapCanvas.addEventListener('mouseup',function(e){if(!dragMoved)handleMapClick(e);dragging=false;mapCanvas.style.cursor='crosshair'});
  mapCanvas.addEventListener('mouseleave',function(){dragging=false;mapCanvas.style.cursor='crosshair';hideTooltip()});
  var lastTouchDist=0;
  mapCanvas.addEventListener('touchstart',function(e){if(e.touches.length===1){dragging=true;dragMoved=false;dragStartX=e.touches[0].clientX;dragStartY=e.touches[0].clientY;dragStartLat=centerLat;dragStartLon=centerLon}else if(e.touches.length===2){lastTouchDist=Math.hypot(e.touches[1].clientX-e.touches[0].clientX,e.touches[1].clientY-e.touches[0].clientY)}e.preventDefault()},{passive:false});
  mapCanvas.addEventListener('touchmove',function(e){if(e.touches.length===1&&dragging){dragMoved=true;var dx=e.touches[0].clientX-dragStartX,dy=e.touches[0].clientY-dragStartY;if(mapMode==='tactical'){var p=5000*zoom;centerLon=dragStartLon-dx/p;centerLat=dragStartLat+dy/p}else{var fz=getOsmZoom(),iz=Math.max(2,Math.min(18,Math.round(fz))),ws=TILE_SIZE*(1<<iz),sf=Math.pow(2,fz-iz);centerLon=dragStartLon-(dx/sf)/ws*360;var origMY=mercY(dragStartLat)*ws,newMY=origMY+(dy/sf);centerLat=Math.atan(Math.sinh(Math.PI*(1-2*newMY/ws)))*180/Math.PI;centerLat=Math.max(-85,Math.min(85,centerLat))}autoFitted=true}else if(e.touches.length===2){var d=Math.hypot(e.touches[1].clientX-e.touches[0].clientX,e.touches[1].clientY-e.touches[0].clientY);if(lastTouchDist>0){zoom=Math.max(0.05,Math.min(10,zoom*(d/lastTouchDist)));autoFitted=true}lastTouchDist=d}e.preventDefault()},{passive:false});
  mapCanvas.addEventListener('touchend',function(){dragging=false;lastTouchDist=0});
  document.addEventListener('keydown',function(e){if(e.target.tagName==='INPUT'||e.target.tagName==='TEXTAREA')return;if(currentPage!=='tactical')return;var s=0.01/zoom;switch(e.key){case'ArrowUp':case'w':centerLat+=s;autoFitted=true;break;case'ArrowDown':case's':centerLat-=s;autoFitted=true;break;case'ArrowLeft':case'a':centerLon-=s;autoFitted=true;break;case'ArrowRight':case'd':centerLon+=s;autoFitted=true;break;case'+':case'=':zoom=Math.min(10,zoom*1.2);autoFitted=true;break;case'-':zoom=Math.max(0.05,zoom/1.2);autoFitted=true;break;case'r':autoFitted=false;break;case'Escape':closeEntityDetail();break}});
}
function selectEntity(id){
  selectedEntity=id;var e=null;for(var i=0;i<entities.length;i++){if(entities[i].id===id){e=entities[i];break}}
  if(!e)return;renderEntityDetail(e);centerLat=e.lat;centerLon=e.lon;autoFitted=true;
  if(currentPage==='tactical')renderFloatEntities();
}
function renderEntityDetail(e){
  var panel=document.getElementById('entityDetail');panel.classList.add('open');
  var il=e.intel_level!==undefined?e.intel_level:4;
  var fc=FACTION_COLORS[e.faction]||'#5a6d8a';var fLabel=FACTION_LABELS[e.faction]||e.faction||'?';
  var tLabel=TYPE_LABELS[e.type]||e.type||'?';var icon=ENT_ICONS[e.type]||'\u25C6';
  var hp=e.health!==undefined?e.health:100;var hpColor=hp>70?'var(--green)':hp>30?'var(--yellow)':'var(--red)';
  var isDead=e.status==='destroyed'||(e.health!==undefined&&e.health<=0);
  var isDamaged=e.status==='damaged'||e.status==='retreating'||(e.health!==undefined&&e.health>0&&e.health<30);
  document.getElementById('edIcon').textContent=icon;document.getElementById('edIcon').style.color=isDead?'var(--muted)':fc;
  document.getElementById('edName').textContent=e.id||'\u2014';
  document.getElementById('edSub').textContent=tLabel+(isDead?' \u2620 \u0130MHA ED\u0130LD\u0130':isDamaged?' \u26A0 HASARLI':'');
  var fBadge=document.getElementById('edFaction');fBadge.textContent=fLabel;fBadge.style.cssText='background:'+fc+';color:'+(e.faction==='enemy'?'#fff':'#000')+';padding:3px 10px;border-radius:10px;font-size:10px;font-weight:700';
  var restricted='<span style="color:var(--muted);font-style:italic">Veri yok</span>';
  var html='';
  html+='<div style="display:flex;align-items:center;gap:8px;margin-bottom:10px;padding:6px 10px;border-radius:6px;background:rgba(255,255,255,.03);border:1px solid '+(INTEL_COLORS[il]||'#555')+'33"><span style="font-size:10px;color:'+(INTEL_COLORS[il]||'#555')+'">\u25C9</span><span style="font-size:10px;color:'+(INTEL_COLORS[il]||'#555')+';font-weight:700">'+(INTEL_LABELS[il]||'?')+'</span><span style="font-size:9px;color:var(--muted);margin-left:auto">\u0130stihbarat Seviyesi '+il+'/4</span></div>';
  if(isDead&&e.status){html+='<div class="ed-section" style="border:1px solid var(--red);border-radius:8px;padding:10px;margin-bottom:8px;background:rgba(255,77,106,.08)"><div style="font-size:13px;font-weight:700;color:var(--red);text-align:center">\u2620 VARLIK \u0130MHA ED\u0130LD\u0130</div><div style="font-size:10px;color:var(--dim);text-align:center;margin-top:4px">Durum: '+(e.status||'destroyed')+'</div></div>'}
  else if(isDamaged){html+='<div class="ed-section" style="border:1px solid var(--yellow);border-radius:8px;padding:10px;margin-bottom:8px;background:rgba(245,166,35,.08)"><div style="font-size:13px;font-weight:700;color:var(--yellow);text-align:center">\u26A0 HASAR TESP\u0130T ED\u0130LD\u0130</div><div style="font-size:10px;color:var(--dim);text-align:center;margin-top:4px">Sa\u011fl\u0131k: %'+(hp!==undefined?hp.toFixed(0):'?')+' | Durum: '+(e.status||'damaged')+'</div></div>'}
  html+='<div class="ed-section"><div class="ed-section-title">Durum Bilgisi</div><div class="ed-grid">';
  html+='<div class="ed-field"><div class="ed-f-val">'+(e.health!==undefined?'<span style="color:'+hpColor+'">%'+hp.toFixed(0)+'</span>':restricted)+'</div><div class="ed-f-lbl">Sa\u011fl\u0131k'+(il<3?' \u{1F512}':'')+'</div></div>';
  html+='<div class="ed-field"><div class="ed-f-val">'+(e.status?'<span style="color:'+(isDead?'var(--red)':isDamaged?'var(--yellow)':'var(--green)')+'">'+(STATUS_LABELS[e.status]||e.status)+'</span>':restricted)+'</div><div class="ed-f-lbl">Operasyonel Durum'+(il<3?' \u{1F512}':'')+'</div></div>';
  html+='<div class="ed-field"><div class="ed-f-val" style="color:'+fc+'">'+fLabel+'</div><div class="ed-f-lbl">Taraf</div></div>';
  html+='<div class="ed-field"><div class="ed-f-val">'+(e.type?tLabel:restricted)+'</div><div class="ed-f-lbl">T\u00fcr / S\u0131n\u0131f'+(il<2?' \u{1F512}':'')+'</div></div></div></div>';
  html+='<div class="ed-section"><div class="ed-section-title">Konum & Hareket</div><div class="ed-grid">';
  html+='<div class="ed-field"><div class="ed-f-val">'+(e.lat?e.lat.toFixed(5)+'\u00b0':'\u2014')+'</div><div class="ed-f-lbl">Enlem</div></div>';
  html+='<div class="ed-field"><div class="ed-f-val">'+(e.lon?e.lon.toFixed(5)+'\u00b0':'\u2014')+'</div><div class="ed-f-lbl">Boylam</div></div>';
  html+='<div class="ed-field"><div class="ed-f-val">'+(e.altitude?e.altitude.toFixed(0)+'m':il>=3?'N/A':restricted)+'</div><div class="ed-f-lbl">\u0130rtifa'+(il<3?' \u{1F512}':'')+'</div></div>';
  html+='<div class="ed-field"><div class="ed-f-val">'+(e.heading!==undefined?e.heading.toFixed(0)+'\u00b0':il>=2?'\u2014':restricted)+'</div><div class="ed-f-lbl">Rota'+(il<2?' \u{1F512}':'')+'</div></div>';
  html+='<div class="ed-field"><div class="ed-f-val">'+(e.speed!==undefined?e.speed.toFixed(1)+' km/h':il>=2?'\u2014':restricted)+'</div><div class="ed-f-lbl">H\u0131z'+(il<2?' \u{1F512}':'')+'</div></div>';
  html+='<div class="ed-field"><div class="ed-f-val">'+(e.sensor_range?e.sensor_range+'m':il>=4?'\u2014':restricted)+'</div><div class="ed-f-lbl">Menzil'+(il<4?' \u{1F512}':'')+'</div></div></div></div>';
  var extra=[];
  if(e.callsign)extra.push(['\u00c7a\u011fr\u0131 \u0130\u015fareti',e.callsign]);if(e.unit)extra.push(['Birlik',e.unit]);if(e.weapon)extra.push(['Silah Sistemi',e.weapon]);
  if(e.fuel!==undefined)extra.push(['Yak\u0131t','%'+e.fuel.toFixed(0)]);if(e.ammo!==undefined)extra.push(['M\u00fchimmat','%'+e.ammo]);
  if(e.sensor_range)extra.push(['Sens\u00f6r Menzili',e.sensor_range+'m']);if(e.comm_freq)extra.push(['Frekans',e.comm_freq+' MHz']);
  if(e.iff_code)extra.push(['IFF Kodu',e.iff_code]);if(e.mission)extra.push(['G\u00f6rev',e.mission]);if(e.threat_level)extra.push(['Tehdit Seviyesi',e.threat_level]);
  if(e.last_update){var lu=typeof e.last_update==='number'?new Date(e.last_update>1e12?e.last_update:e.last_update*1000):new Date(e.last_update);extra.push(['Son G\u00fcncelleme',lu.toLocaleString('tr-TR')])}
  var known={id:1,type:1,faction:1,lat:1,lon:1,health:1,heading:1,speed:1,altitude:1,alt:1,status:1,range:1,callsign:1,unit:1,weapon:1,fuel:1,ammo:1,sensor_range:1,comm_freq:1,iff_code:1,mission:1,threat_level:1,last_update:1,updated_at:1,created_at:1,intel_level:1};
  for(var k in e){if(!known[k]&&e[k]!==undefined&&e[k]!==null&&e[k]!==''){var dv=String(e[k]);if(typeof e[k]==='number'&&e[k]>1e11&&e[k]<2e13)dv=new Date(e[k]).toLocaleString('tr-TR');extra.push([k,dv])}}
  if(extra.length>0){html+='<div class="ed-section"><div class="ed-section-title">Detay Bilgileri'+(il<4?' <span style="font-size:9px;color:var(--muted)">(k\u0131s\u0131tl\u0131)</span>':'')+'</div><div class="ed-grid">';for(var i=0;i<extra.length;i++)html+='<div class="ed-field"><div class="ed-f-val">'+extra[i][1]+'</div><div class="ed-f-lbl">'+extra[i][0]+'</div></div>';html+='</div></div>'}
  if(e.faction==='enemy'){var tLabel2=TYPE_LABELS[e.type]||e.type||'?';html+='<div class="ed-section"><div class="ed-section-title" style="color:var(--red)">\u26A0 Tehdit De\u011ferlendirmesi</div><div style="background:rgba(255,77,106,.08);border:1px solid rgba(255,77,106,.2);border-radius:6px;padding:10px"><div style="font-size:12px;font-weight:700;color:var(--red);margin-bottom:4px">D\u00dc\u015eMAN VARLIK TESP\u0130T ED\u0130LD\u0130</div><div style="font-size:10px;color:var(--dim);line-height:1.5">T\u00fcr: <b style="color:var(--text)">'+tLabel2+'</b><br>Konum: <b style="color:var(--text)">'+e.lat.toFixed(4)+'N, '+e.lon.toFixed(4)+'E</b><br>';if(e.speed)html+='H\u0131z: <b style="color:var(--text)">'+e.speed.toFixed(1)+' km/h</b> \u2014 ';if(e.heading)html+='Rota: <b style="color:var(--text)">'+e.heading.toFixed(0)+'\u00b0</b>';html+='</div></div></div>'}
  document.getElementById('edBody').innerHTML=html;
}
function closeEntityDetail(){document.getElementById('entityDetail').classList.remove('open');selectedEntity=null;if(currentPage==='tactical')renderFloatEntities()}
function getOsmZoom(){return Math.log2(2000*zoom)+5}
function mercY(lat){return(1-Math.log(Math.tan(lat*Math.PI/180)+1/Math.cos(lat*Math.PI/180))/Math.PI)/2}
function toScreen(lat,lon,W,H){
  if(mapMode==='tactical'){var px=((lon-centerLon)*5000*zoom)+W/2;var py=((centerLat-lat)*5000*zoom)+H/2;return{x:px,y:py}}
  else{var fz=getOsmZoom();var iz=Math.max(2,Math.min(18,Math.round(fz)));var ws=TILE_SIZE*(1<<iz);var sf=Math.pow(2,fz-iz);
    var cx01=centerLon/360+0.5,cy01=mercY(centerLat);var ex01=lon/360+0.5,ey01=mercY(lat);
    var px=W/2+(ex01-cx01)*ws*sf;var py=H/2+(ey01-cy01)*ws*sf;return{x:px,y:py}}
}
function screenToLatLon(mx,my,W,H){if(mapMode==='tactical'){var s=5000*zoom;return{lat:centerLat-(my-H/2)/s,lon:centerLon+(mx-W/2)/s}}else{var fz=getOsmZoom(),iz=Math.max(2,Math.min(18,Math.round(fz))),ws=TILE_SIZE*(1<<iz),sf=Math.pow(2,fz-iz),cx01=centerLon/360+0.5,cy01=mercY(centerLat),ex01=cx01+(mx-W/2)/(ws*sf),ey01=cy01+(my-H/2)/(ws*sf);return{lat:Math.atan(Math.sinh(Math.PI*(1-2*ey01)))*180/Math.PI,lon:(ex01-0.5)*360}}}
function renderLoop(){requestAnimationFrame(renderLoop);if(currentPage==='tactical')renderTacticalCanvas();else if(currentPage==='overview')renderMiniMapCanvas()}
function renderTacticalCanvas(){
  var W=mapW,H=mapH;if(W===0||H===0)return;mapCtx.clearRect(0,0,W,H);
  if(mapMode==='tactical'){mapCtx.fillStyle='#0a1628';mapCtx.fillRect(0,0,W,H);drawGrid(mapCtx,W,H);drawEntitiesOnMap(mapCtx,W,H)}
  else{drawTiles(mapCtx,W,H);drawEntitiesOnMap(mapCtx,W,H)}
  if(!autoFitted&&entities.length>0){fitToEntities();autoFitted=true}
  mapCtx.fillStyle='rgba(0,0,0,0.6)';mapCtx.fillRect(8,H-50,300,42);mapCtx.fillStyle='#66d9ef';mapCtx.font='10px Inter,monospace';
  mapCtx.fillText('Enlem: '+centerLat.toFixed(5)+' | Boylam: '+centerLon.toFixed(5),14,H-34);
  mapCtx.fillText('Zoom: '+zoom.toFixed(2)+(mapMode!=='tactical'?' (OSM z'+getOsmZoom().toFixed(1)+')':'')+' | Varl\u0131k: '+entities.length,14,H-18);
  document.getElementById('mapInfo').textContent=(mapMode==='tactical'?'Taktik Mod':mapMode==='satellite'?'Uydu G\u00f6r\u00fcn\u00fcm\u00fc':'Harita G\u00f6r\u00fcn\u00fcm\u00fc')+' \u2014 '+entities.length+' varl\u0131k';
}
function renderMiniMapCanvas(){
  var W=miniW,H=miniH;if(W===0||H===0)return;miniMapCtx.clearRect(0,0,W,H);
  miniMapCtx.fillStyle='#0a1628';miniMapCtx.fillRect(0,0,W,H);
  miniMapCtx.strokeStyle='rgba(0,255,200,0.04)';miniMapCtx.lineWidth=0.5;var step=40;
  for(var x=0;x<W;x+=step){miniMapCtx.beginPath();miniMapCtx.moveTo(x,0);miniMapCtx.lineTo(x,H);miniMapCtx.stroke()}
  for(var y=0;y<H;y+=step){miniMapCtx.beginPath();miniMapCtx.moveTo(0,y);miniMapCtx.lineTo(W,y);miniMapCtx.stroke()}
  if(entities.length===0)return;
  var minLat=90,maxLat=-90,minLon=180,maxLon=-180;
  for(var i=0;i<entities.length;i++){minLat=Math.min(minLat,entities[i].lat);maxLat=Math.max(maxLat,entities[i].lat);minLon=Math.min(minLon,entities[i].lon);maxLon=Math.max(maxLon,entities[i].lon)}
  var mCLat=(minLat+maxLat)/2,mCLon=(minLon+maxLon)/2;var span=Math.max(maxLat-minLat,maxLon-minLon,0.001);var mZoom=Math.min(2,0.015/span);
  for(var i=0;i<entities.length;i++){var e=entities[i];
    var px=((e.lon-mCLon)*5000*mZoom)+W/2;var py=((mCLat-e.lat)*5000*mZoom)+H/2;
    if(px<-20||px>W+20||py<-20||py>H+20)continue;
    var fc=e.faction||'unknown';var clr=fc==='friendly'?'#00e5a0':fc==='enemy'?'#ff4d6a':fc==='neutral'?'#f5a623':'#8b95a5';
    var t=e.type||'';var isAir=t.indexOf('drone')>=0||t.indexOf('uav')>=0||t.indexOf('aircraft')>=0||t.indexOf('helicopter')>=0||t.indexOf('fighter')>=0;
    var isVehicle=t.indexOf('tank')>=0||t.indexOf('apc')>=0||t.indexOf('vehicle')>=0||t.indexOf('humvee')>=0||t.indexOf('truck')>=0;
    var sz=5;miniMapCtx.fillStyle=clr;miniMapCtx.strokeStyle='rgba(0,0,0,0.6)';miniMapCtx.lineWidth=0.5;
    if(isAir){miniMapCtx.beginPath();miniMapCtx.moveTo(px,py-sz);miniMapCtx.lineTo(px+sz*0.7,py+sz*0.5);miniMapCtx.lineTo(px-sz*0.7,py+sz*0.5);miniMapCtx.closePath();miniMapCtx.fill();miniMapCtx.stroke()}
    else if(isVehicle){miniMapCtx.fillRect(px-sz*0.5,py-sz*0.4,sz,sz*0.8);miniMapCtx.strokeRect(px-sz*0.5,py-sz*0.4,sz,sz*0.8)}
    else{miniMapCtx.beginPath();miniMapCtx.arc(px,py,sz*0.5,0,Math.PI*2);miniMapCtx.fill();miniMapCtx.stroke()}
  }
}
function fitToEntities(){
  if(entities.length===0)return;var minLat=90,maxLat=-90,minLon=180,maxLon=-180;
  for(var i=0;i<entities.length;i++){minLat=Math.min(minLat,entities[i].lat);maxLat=Math.max(maxLat,entities[i].lat);minLon=Math.min(minLon,entities[i].lon);maxLon=Math.max(maxLon,entities[i].lon)}
  centerLat=(minLat+maxLat)/2;centerLon=(minLon+maxLon)/2;var span=Math.max(maxLat-minLat,maxLon-minLon,0.001);
  zoom=mapMode==='tactical'?Math.min(2,0.02/span):Math.min(10,0.5/span);
}
function drawGrid(ctx,W,H){ctx.strokeStyle='rgba(0,255,200,0.05)';ctx.lineWidth=0.5;var step=50;for(var x=0;x<W;x+=step){ctx.beginPath();ctx.moveTo(x,0);ctx.lineTo(x,H);ctx.stroke()}for(var y=0;y<H;y+=step){ctx.beginPath();ctx.moveTo(0,y);ctx.lineTo(W,y);ctx.stroke()}}
function drawTiles(ctx,W,H){
  var fz=getOsmZoom();var iz=Math.max(2,Math.min(18,Math.round(fz)));var nTiles=1<<iz;var sf=Math.pow(2,fz-iz);
  var cx01=centerLon/360+0.5,cy01=mercY(centerLat);var tileSize=TILE_SIZE*sf;var cxp=cx01*nTiles,cyp=cy01*nTiles;
  var offX=W/2-cxp*tileSize,offY=H/2-cyp*tileSize;
  var sX=Math.floor(-offX/tileSize)-1,eX=Math.ceil((W-offX)/tileSize)+1;
  var sY=Math.floor(-offY/tileSize)-1,eY=Math.ceil((H-offY)/tileSize)+1;
  var urlT=mapMode==='satellite'?TILE_SERVERS.satellite:TILE_SERVERS.map;
  for(var tx=sX;tx<=eX;tx++){for(var ty=sY;ty<=eY;ty++){var wr=((tx%nTiles)+nTiles)%nTiles;if(ty<0||ty>=nTiles)continue;
    var drawX=offX+tx*tileSize,drawY=offY+ty*tileSize;var key=iz+'/'+wr+'/'+ty;
    if(tileCache[key]){ctx.drawImage(tileCache[key],drawX,drawY,tileSize,tileSize)}
    else{ctx.fillStyle='#0d1117';ctx.fillRect(drawX,drawY,tileSize,tileSize);var img=new Image();img.crossOrigin='anonymous';img.onload=(function(k,im){return function(){tileCache[k]=im}})(key,img);img.src=urlT.replace('{z}',iz).replace('{x}',wr).replace('{y}',ty)}}}
}
function drawEntitiesOnMap(ctx,W,H){
  var now=Date.now();var entLayer=null;for(var i=0;i<layers.length;i++){if(layers[i].id==='entities'){entLayer=layers[i];break}}
  if(entLayer&&!entLayer.visible)return;
  for(var idx=0;idx<entities.length;idx++){var e=entities[idx];var p=toScreen(e.lat,e.lon,W,H);
    if(p.x<-50||p.x>W+50||p.y<-50||p.y>H+50)continue;
    var fc=e.faction||'unknown';var clr=fc==='friendly'?'#00e5a0':fc==='enemy'?'#ff4d6a':fc==='neutral'?'#f5a623':'#8b95a5';
    var isSelected=e.id===selectedEntity,isHovered=e.id===hoveredEntity;var sz=isSelected?14:isHovered?12:10;
    var isDead=e.status==='destroyed'||(e.health!==undefined&&e.health<=0);
    if(isDead){ctx.save();ctx.globalAlpha=0.35}
    if(isSelected){ctx.beginPath();var pulse2=Math.sin(now/300)*4+20;ctx.arc(p.x,p.y,pulse2,0,Math.PI*2);ctx.strokeStyle=clr+'44';ctx.lineWidth=2;ctx.stroke();ctx.beginPath();ctx.arc(p.x,p.y,16,0,Math.PI*2);ctx.strokeStyle=clr;ctx.lineWidth=1.5;ctx.setLineDash([3,3]);ctx.stroke();ctx.setLineDash([])}
    var t=e.type||'';var isAir=t.indexOf('drone')>=0||t.indexOf('uav')>=0||t.indexOf('aircraft')>=0||t.indexOf('helicopter')>=0||t.indexOf('fighter')>=0;
    var isVehicle=t.indexOf('tank')>=0||t.indexOf('apc')>=0||t.indexOf('vehicle')>=0||t.indexOf('humvee')>=0||t.indexOf('truck')>=0;
    var isInstall=t.indexOf('radar')>=0||t.indexOf('sam')>=0||t.indexOf('artillery')>=0||t.indexOf('command')>=0||t.indexOf('base')>=0||t.indexOf('sensor')>=0;
    ctx.save();ctx.fillStyle=clr;ctx.strokeStyle='rgba(0,0,0,0.8)';ctx.lineWidth=1;
    if(isAir){ctx.beginPath();ctx.moveTo(p.x,p.y-sz);ctx.lineTo(p.x+sz*0.8,p.y+sz*0.6);ctx.lineTo(p.x,p.y+sz*0.3);ctx.lineTo(p.x-sz*0.8,p.y+sz*0.6);ctx.closePath();ctx.fill();ctx.stroke()}
    else if(isVehicle){ctx.fillRect(p.x-sz*0.6,p.y-sz*0.5,sz*1.2,sz);ctx.strokeRect(p.x-sz*0.6,p.y-sz*0.5,sz*1.2,sz);if(t.indexOf('tank')>=0){ctx.beginPath();ctx.moveTo(p.x,p.y);ctx.lineTo(p.x+sz*0.8,p.y-sz*0.8);ctx.strokeStyle=clr;ctx.lineWidth=2;ctx.stroke()}}
    else if(isInstall){ctx.beginPath();ctx.moveTo(p.x,p.y-sz);ctx.lineTo(p.x+sz,p.y);ctx.lineTo(p.x,p.y+sz);ctx.lineTo(p.x-sz,p.y);ctx.closePath();ctx.fill();ctx.stroke()}
    else{ctx.beginPath();ctx.arc(p.x,p.y,sz*0.6,0,Math.PI*2);ctx.fill();ctx.stroke()}
    ctx.restore();
    if(isDead){ctx.save();ctx.strokeStyle='#ff4d6a';ctx.lineWidth=2;ctx.beginPath();ctx.moveTo(p.x-6,p.y-6);ctx.lineTo(p.x+6,p.y+6);ctx.moveTo(p.x+6,p.y-6);ctx.lineTo(p.x-6,p.y+6);ctx.stroke();ctx.restore();ctx.restore();continue}
    if((isSelected||isHovered)&&e.speed&&e.heading){var rad=(e.heading-90)*Math.PI/180;var vlen=Math.min(40,e.speed);ctx.save();ctx.beginPath();ctx.moveTo(p.x,p.y);ctx.lineTo(p.x+Math.cos(rad)*vlen,p.y+Math.sin(rad)*vlen);ctx.strokeStyle=clr+'99';ctx.lineWidth=2;ctx.setLineDash([4,3]);ctx.stroke();ctx.setLineDash([]);ctx.restore()}
    var hp=e.health!==undefined?e.health:100;var hpClr=hp>70?'#00e5a0':hp>30?'#f5a623':'#ff4d6a';var barW=24,barH=3;
    ctx.fillStyle='rgba(0,0,0,0.5)';ctx.fillRect(p.x-barW/2,p.y+sz+4,barW,barH);ctx.fillStyle=hpClr;ctx.fillRect(p.x-barW/2,p.y+sz+4,barW*Math.max(0,hp)/100,barH);
    ctx.fillStyle='#fff';ctx.font=(isSelected?'bold 11px':'10px')+' Inter,sans-serif';ctx.textAlign='center';
    var lbl=e.id||(TYPE_LABELS[e.type]||e.type||'?');ctx.save();var tw=ctx.measureText(lbl).width;ctx.fillStyle='rgba(0,0,0,0.6)';
    if(ctx.roundRect){ctx.beginPath();ctx.roundRect(p.x-tw/2-4,p.y+sz+9,tw+8,14,3);ctx.fill()}else{ctx.fillRect(p.x-tw/2-4,p.y+sz+9,tw+8,14)}
    ctx.restore();ctx.fillStyle=isSelected?clr:'#e0e6ed';ctx.fillText(lbl,p.x,p.y+sz+20);
  }
}
function updateHover(e){var rect=mapCanvas.getBoundingClientRect(),mx=e.clientX-rect.left,my=e.clientY-rect.top,W=mapW,H=mapH;var closest=null,closestDist=Infinity;for(var i=0;i<entities.length;i++){var ent=entities[i];var p=toScreen(ent.lat,ent.lon,W,H);var d=Math.hypot(p.x-mx,p.y-my);if(d<20&&d<closestDist){closest=ent;closestDist=d}}if(closest){hoveredEntity=closest.id;mapCanvas.style.cursor='pointer';showTooltip(closest,e.clientX,e.clientY)}else{hoveredEntity=null;mapCanvas.style.cursor='crosshair';hideTooltip()}}
function showTooltip(e,mx,my){
  var tt=document.getElementById('mapTooltip');var il=e.intel_level!==undefined?e.intel_level:4;
  var fc=FACTION_COLORS[e.faction]||'#5a6d8a';var fLabel=FACTION_LABELS[e.faction]||'?';var tLabel=TYPE_LABELS[e.type]||e.type||'?';
  var icon=ENT_ICONS[e.type]||'\u25C6';var hp=e.health!==undefined?e.health:100;var hpColor=hp>70?'var(--green)':hp>30?'var(--yellow)':'var(--red)';
  var isDead=e.status==='destroyed'||(e.health!==undefined&&e.health<=0);
  var html='<div class="tt-header"><span class="tt-icon" style="color:'+(isDead?'var(--muted)':fc)+'">'+icon+'</span><span class="tt-name">'+(e.id||'?')+(isDead?' \u2620':'')+'</span><span class="tt-faction" style="background:'+fc+';color:'+(e.faction==='enemy'?'#fff':'#000')+'">'+fLabel+'</span></div>';
  html+='<div class="tt-row" style="border-bottom:1px solid rgba(255,255,255,.05);padding-bottom:3px;margin-bottom:2px"><span style="color:'+(INTEL_COLORS[il]||'#555')+'">\u25C9 '+INTEL_LABELS[il]+'</span><span class="tt-val" style="color:'+(INTEL_COLORS[il]||'#555')+'">'+il+'/4</span></div>';
  if(il>=2)html+='<div class="tt-row"><span>T\u00fcr</span><span class="tt-val">'+tLabel+'</span></div>';
  if(e.status)html+='<div class="tt-row"><span>Durum</span><span class="tt-val" style="color:'+(isDead?'var(--red)':'var(--text)')+'">'+(e.status||'active')+'</span></div>';
  if(e.health!==undefined)html+='<div class="tt-row"><span>Sa\u011fl\u0131k</span><span class="tt-val" style="color:'+hpColor+'">%'+hp.toFixed(0)+'</span></div>';
  html+='<div class="tt-row"><span>Konum</span><span class="tt-val">'+(e.lat?e.lat.toFixed(4):'?')+'N, '+(e.lon?e.lon.toFixed(4):'?')+'E</span></div>';
  if(e.speed!==undefined)html+='<div class="tt-row"><span>H\u0131z</span><span class="tt-val">'+e.speed.toFixed(1)+' km/h</span></div>';
  if(e.heading!==undefined)html+='<div class="tt-row"><span>Rota</span><span class="tt-val">'+e.heading.toFixed(0)+'\u00b0</span></div>';
  if(e.fuel!==undefined)html+='<div class="tt-row"><span>Yak\u0131t</span><span class="tt-val">%'+e.fuel.toFixed(0)+'</span></div>';
  if(il<4)html+='<div style="font-size:8px;color:var(--muted);text-align:center;margin-top:4px;border-top:1px solid rgba(255,255,255,.05);padding-top:3px">\u{1F512} S\u0131n\u0131rl\u0131 istihbarat</div>';
  tt.innerHTML=html;tt.style.display='block';var tw2=240;var tx=mx+12,ty=my+12;if(tx+tw2>window.innerWidth)tx=mx-tw2-8;if(ty+180>window.innerHeight)ty=my-180;if(ty<0)ty=4;if(tx<0)tx=4;tt.style.left=tx+'px';tt.style.top=ty+'px';
}
function hideTooltip(){document.getElementById('mapTooltip').style.display='none'}
function handleMapClick(e){var rect=mapCanvas.getBoundingClientRect(),mx=e.clientX-rect.left,my=e.clientY-rect.top,W=mapW,H=mapH;var closest=null,closestDist=Infinity;for(var i=0;i<entities.length;i++){var ent=entities[i];var p=toScreen(ent.lat,ent.lon,W,H);var d=Math.hypot(p.x-mx,p.y-my);if(d<20&&d<closestDist){closest=ent;closestDist=d}}if(closest){selectEntity(closest.id);hideTooltip()}else{closeEntityDetail()}}
function toggleLayer(id){var l=null;for(var i=0;i<layers.length;i++){if(layers[i].id===id){l=layers[i];break}}if(l){l.visible=!l.visible;fetch(API+'/layers/'+id,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({visible:l.visible})});renderFloatLayers()}}
function setMapMode(mode){mapMode=mode;var btns=document.querySelectorAll('.map-btn');for(var i=0;i<btns.length;i++){btns[i].className='map-btn'+(btns[i].getAttribute('data-mode')===mode?' active':'')}}
window.addEventListener('load',init);
</script>
</body>
</html>
`

// DashboardHandler returns the HTML handler
func DashboardHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(dashboardHTML))
	}
}
