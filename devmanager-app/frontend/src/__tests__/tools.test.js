import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mountTools } from '../panels/tools.js';

// Test del panel tools (Issue #68): browser, editor y snippets con el mock
// de window.go. La paleta de comandos se cubre por snap de render básico.
function buildDOM() {
    document.body.innerHTML = `
        <input id="tools-search">
        <button id="tools-palette"></button>
        <div id="tools-result" hidden></div>
        <div id="tools-crumbs"></div>
        <div id="tools-list"></div>
        <div id="tools-search-results" hidden></div>
        <span id="tools-editor-path"></span>
        <button id="tools-save" disabled></button>
        <button id="tools-close" disabled></button>
        <textarea id="tools-editor" hidden></textarea>
        <div id="tools-editor-empty"></div>
        <input id="tools-snippet-name">
        <input id="tools-snippet-cmd">
        <button id="tools-snippet-add"></button>
        <div id="tools-snippets"></div>
        <div id="tools-snippets-empty"></div>`;
}

const devMock = {
    DevListDir: vi.fn(async () => [
        { name: 'src', rel: 'src', isDir: true, size: 0 },
        { name: 'package.json', rel: 'package.json', isDir: false, size: 24 },
    ]),
    DevReadFile: vi.fn(async () => 'const a = 1;\n'),
    DevWriteFile: vi.fn(async () => null),
    DevSearchFiles: vi.fn(async () => [
        { path: 'src/app.js', line: 1, preview: 'const a = 1;' },
    ]),
    DevGetSnippets: vi.fn(async () => [
        { id: 'run-dev', name: 'Run dev', command: 'npm run dev' },
    ]),
    DevSaveSnippet: vi.fn(async () => null),
    DevDeleteSnippet: vi.fn(async () => null),
};

function stubCtx() {
    return {
        $: (id) => document.getElementById(id),
        api: { runScript: vi.fn(async () => null) },
        events: () => ({ EventsOn: vi.fn() }),
        selectedIndex: () => 0,
    };
}

function tick() {
    return new Promise((resolve) => setTimeout(resolve, 0));
}

describe('tools panel', () => {
    beforeEach(() => {
        buildDOM();
        vi.clearAllMocks();
        window.go = { main: { App: devMock } };
    });

    it('lista el directorio raíz al cambiar de proyecto', async () => {
        const panel = mountTools(stubCtx());
        panel.onProjectChanged({ name: 'p' });
        await tick();
        expect(devMock.DevListDir).toHaveBeenCalledWith(0, '.');
        expect(document.getElementById('tools-list').textContent).toContain('src');
        expect(document.getElementById('tools-list').textContent).toContain('package.json');
        // Breadcrumb con la raíz.
        expect(document.getElementById('tools-crumbs').textContent).toContain('project root');
    });

    it('abre un archivo en el editor y marca dirty al editar', async () => {
        const panel = mountTools(stubCtx());
        panel.onProjectChanged({ name: 'p' });
        await tick();
        // Click en el archivo (segunda fila).
        const rows = [...document.querySelectorAll('#tools-list .tools-row')];
        rows[1].click();
        await tick();
        expect(devMock.DevReadFile).toHaveBeenCalledWith(0, 'package.json');
        const editor = document.getElementById('tools-editor');
        expect(editor.hidden).toBe(false);
        expect(editor.value).toBe('const a = 1;\n');
        expect(document.getElementById('tools-save').disabled).toBe(true);
        editor.value = 'const a = 2;\n';
        editor.dispatchEvent(new Event('input'));
        expect(document.getElementById('tools-save').disabled).toBe(false);
        expect(document.getElementById('tools-editor-path').textContent).toContain('*');
    });

    it('guarda cambios vía DevWriteFile', async () => {
        const panel = mountTools(stubCtx());
        panel.onProjectChanged({ name: 'p' });
        await tick();
        const rows = [...document.querySelectorAll('#tools-list .tools-row')];
        rows[1].click();
        await tick();
        const editor = document.getElementById('tools-editor');
        editor.value = 'nuevo';
        editor.dispatchEvent(new Event('input'));
        document.getElementById('tools-save').click();
        await tick();
        expect(devMock.DevWriteFile).toHaveBeenCalledWith(0, 'package.json', 'nuevo');
    });

    it('busca archivos y muestra resultados', async () => {
        const panel = mountTools(stubCtx());
        panel.onProjectChanged({ name: 'p' });
        await tick();
        const search = document.getElementById('tools-search');
        search.value = 'app.js';
        search.dispatchEvent(new Event('input'));
        await new Promise((r) => setTimeout(r, 350));
        expect(devMock.DevSearchFiles).toHaveBeenCalledWith(0, 'app.js');
        expect(document.getElementById('tools-search-results').textContent).toContain('src/app.js');
    });

    it('añade un snippet y refresca la lista', async () => {
        const panel = mountTools(stubCtx());
        panel.onProjectChanged({ name: 'p' });
        await tick();
        document.getElementById('tools-snippet-name').value = 'Lint';
        document.getElementById('tools-snippet-cmd').value = 'npm run lint';
        document.getElementById('tools-snippet-add').click();
        await tick();
        expect(devMock.DevSaveSnippet).toHaveBeenCalledWith({ id: '', name: 'Lint', command: 'npm run lint' });
        expect(document.getElementById('tools-snippets').textContent).toContain('Run dev');
    });

    it('la paleta abre overlay con comandos y snippets', async () => {
        const panel = mountTools(stubCtx());
        panel.onProjectChanged({ name: 'p' });
        await tick();
        document.getElementById('tools-palette').click();
        const overlay = document.getElementById('tools-palette-overlay');
        expect(overlay).not.toBeNull();
        const items = [...overlay.querySelectorAll('.palette-item')].map((b) => b.textContent);
        expect(items).toContain('Run tests (Playwright)');
        expect(items).toContain('Snippet: Run dev');
        overlay.remove();
    });
});
