// Helpers multi-env Fase 1 (#67): puros, sin dependencias.
// Convención snake_case del backend: active_env / envs.

const ENV_RE = /^[a-z0-9_-]{1,32}$/;
const VAR_RE = /^[A-Z_][A-Z0-9_]*$/;

export function isValidEnvName(name) {
    return ENV_RE.test(name || '');
}

export function isValidVarKey(key) {
    return VAR_RE.test(key || '');
}

// MASKED es el placeholder de secretos (paridad Go models.MaskedValue).
// Nunca comparar valores reales contra esto salvo para preservarlos al guardar.
export const MASKED = '***';

export function isSecretKey(secrets, key) {
    return (secrets || []).includes(key);
}

// defaultEnvFile paridad Go models.DefaultEnvFile.
export function defaultEnvFile(name) {
    if (name === 'dev') return '.env';
    if (name === 'staging') return '.env.staging';
    if (name === 'prod') return '.env.prod';
    return `.env.${name}`;
}

export function envNames(project) {
    if (!project || !project.envs) return [];
    return Object.keys(project.envs).sort();
}

export function activeEnvOf(project) {
    if (!project) return '';
    return project.active_env || '';
}

// normalizeEnv completa vars/env_file/secrets con defaults (paridad Go ensureEnvs).
export function normalizeEnv(e, name) {
    const src = e || {};
    return {
        server: { ...(src.server || {}) },
        playwright: { ...(src.playwright || {}) },
        user: { ...(src.user || {}) },
        vars: { ...(src.vars || {}) },
        env_file: src.env_file || defaultEnvFile(name),
        secrets: [...(src.secrets || [])],
    };
}

// ensureEnvs sintetiza dev desde top-level (paridad Go ensureEnvs).
// Normaliza vars/env_file. No muta: devuelve { active_env, envs }.
export function ensureEnvs(project) {
    const p = project || {};
    if (p.envs && Object.keys(p.envs).length > 0) {
        const active = p.active_env && p.envs[p.active_env] ? p.active_env
            : (p.envs.dev ? 'dev' : Object.keys(p.envs).sort()[0]);
        const envs = {};
        Object.keys(p.envs).forEach((n) => { envs[n] = normalizeEnv(p.envs[n], n); });
        return { active_env: active, envs };
    }
    const dev = normalizeEnv({
        server: { ...(p.server || {}) },
        playwright: { ...(p.playwright || {}) },
        user: { ...(p.user || {}) },
    }, 'dev');
    return { active_env: 'dev', envs: { dev } };
}

export function effectiveEnv(project) {
    const ensured = ensureEnvs(project);
    return ensured.envs[ensured.active_env] || { server: {}, playwright: {}, user: {} };
}

export function effectiveServer(project) {
    const env = effectiveEnv(project);
    if (env.server && Object.keys(env.server).length > 0) return env.server;
    return (project && project.server) || {};
}

export function isServerRunning(serverState) {
    return serverState === 'running' || serverState === 'starting' || serverState === 'stopping';
}
