// Panel de dependencias (Issue #61/#62): dashboard, outdated, audit.

const SEVERITY_CLASS = {
    low: 'vuln-low', moderate: 'vuln-moderate',
    high: 'vuln-high', critical: 'vuln-critical',
};

export function mount(ctx) {
    const { $, api } = ctx;

    function renderAudit(audit) {
        const box = $('deps-audit');
        box.innerHTML = '';
        if (audit && audit.error) {
            box.hidden = false;
            const row = document.createElement('div');
            row.textContent = `Audit no disponible: ${audit.error}`;
            box.appendChild(row);
            return;
        }
        const vulns = (audit && audit.vulns) || [];
        if (vulns.length === 0) {
            box.hidden = true;
            return;
        }
        box.hidden = false;
        const header = document.createElement('div');
        header.textContent = `Audit ${audit.manager}: ${vulns.length} vulnerabilidad(es)`;
        box.appendChild(header);
        vulns.forEach((v) => {
            const row = document.createElement('div');
            row.className = `audit-vuln ${SEVERITY_CLASS[v.severity] || 'vuln-low'}`;
            row.textContent = `[${v.severity}] ${v.name} — ${v.title}`;
            box.appendChild(row);
        });
    }

    function render(result) {
        const list = $('deps-list');
        const empty = $('deps-empty');
        list.innerHTML = '';
        const deps = (result && result.deps) || [];
        if (!result || result.manager === '' || deps.length === 0) {
            empty.hidden = false;
            $('deps-run-audit').disabled = true;
            empty.textContent = result && result.error
                ? `Error: ${result.error}`
                : 'Sin dependencias o proyecto sin gestor de paquetes.';
            return;
        }
        empty.hidden = true;
        $('deps-run-audit').disabled = false;
        deps.forEach((d) => {
            const row = document.createElement('div');
            row.className = 'dep-row' + (d.outdated ? ' dep-outdated' : '');
            const name = document.createElement('span');
            name.className = 'dep-name';
            name.textContent = d.name;
            row.appendChild(name);
            const ver = document.createElement('span');
            ver.className = 'dep-vers';
            if (d.outdated) {
                const badge = document.createElement('span');
                badge.className = 'dep-badge';
                badge.textContent = 'outdated';
                row.appendChild(badge);
                ver.textContent = `${d.current} → ${d.latest}`;
            } else {
                ver.textContent = d.current;
            }
            row.appendChild(ver);
            list.appendChild(row);
        });
    }

    async function refresh() {
        const i = ctx.selectedIndex();
        if (i < 0) return;
        render(await api.getDeps(i));
    }

    $('deps-run-audit').addEventListener('click', async () => {
        const i = ctx.selectedIndex();
        if (i < 0) return;
        renderAudit(await api.getDepsAudit(i));
    });

    return {
        onProjectChanged: refresh,
        refresh,
    };
}