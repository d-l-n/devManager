// Iconos importados de Reicon
// Para usar en el frontend, puedes importarlos así:
// import { serverIcon } from './icons/index.js';

export const logoIcon = () => import('./logo.svg?raw');
export const serverIcon = () => import('./server.svg?raw');
export const playIcon = () => import('./play.svg?raw');
export const codeIcon = () => import('./code.svg?raw');
export const gitBranchIcon = () => import('./git-branch.svg?raw');
export const monitorIcon = () => import('./monitor.svg?raw');
export const bugIcon = () => import('./bug.svg?raw');
export const folderIcon = () => import('./folder.svg?raw');
export const settingsIcon = () => import('./settings.svg?raw');

// También puedes importar directamente los SVG si tu configuración lo permite
export { default as logoSvg } from './logo.svg';
export { default as serverSvg } from './server.svg';
export { default as playSvg } from './play.svg';
export { default as codeSvg } from './code.svg';
export { default as gitBranchSvg } from './git-branch.svg';
export { default as monitorSvg } from './monitor.svg';
export { default as bugSvg } from './bug.svg';
export { default as folderSvg } from './folder.svg';
export { default as settingsSvg } from './settings.svg';