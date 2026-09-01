#!/usr/bin/env node

/**
 * Cross-platform build script for devManager Desktop Application
 * Replaces build-desktop.bat with support for Windows, macOS, and Linux
 * 
 * Usage: node build.js [options]
 * 
 * Options:
 *   --clean, -c        Clean build directories before building
 *   --debug, -d        Build in debug mode (default: release)
 *   --parallel, -p     Enable parallel builds where possible
 *   --no-version-check Skip version checking
 *   --verbose, -v       Verbose output
 *   --help, -h         Show this help message
 */

import { execSync, spawn } from 'child_process';
import { existsSync, readFileSync, rmSync, mkdirSync, statSync, writeFileSync } from 'fs';
import { join, dirname, resolve } from 'path';
import { fileURLToPath } from 'url';
import { createHash } from 'crypto';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// Configuration
const CONFIG = {
  MIN_NODE_VERSION: '18.0.0',
  MIN_GO_VERSION: '1.21.0',
  MIN_WAILS_VERSION: '2.15.0',
  REQUIRED_DISK_SPACE_GB: 2,
  BUILD_ID: generateBuildId(),
  PLATFORM: process.platform,
  ARCH: process.arch,
};

// Parse command line arguments
const args = process.argv.slice(2);
const options = {
  clean: false,
  debug: false,
  parallel: false,
  versionCheck: true,
  verbose: false,
  help: false,
};

for (let i = 0; i < args.length; i++) {
  const arg = args[i];
  switch (arg) {
    case '--clean':
    case '-c':
      options.clean = true;
      break;
    case '--debug':
    case '-d':
      options.debug = true;
      break;
    case '--parallel':
    case '-p':
      options.parallel = true;
      break;
    case '--no-version-check':
      options.versionCheck = false;
      break;
    case '--verbose':
    case '-v':
      options.verbose = true;
      break;
    case '--help':
    case '-h':
      options.help = true;
      break;
    default:
      console.error(`Unknown option: ${arg}`);
      options.help = true;
  }
}

if (options.help) {
  showHelp();
  process.exit(0);
}

// Main build function
async function main() {
  const startTime = Date.now();
  
  try {
    logHeader();
    await checkSystemRequirements();
    await checkDependencies();
    await setupBuildEnvironment();
    await buildFrontend();
    await buildApplication();
    await verifyBuild();
    
    const duration = Math.round((Date.now() - startTime) / 1000);
    const minutes = Math.floor(duration / 60);
    const seconds = duration % 60;
    
    console.log('\n========================================');
    console.log('BUILD COMPLETED SUCCESSFULLY!');
    console.log('========================================');
    console.log(`Build ID: ${CONFIG.BUILD_ID}`);
    console.log(`Platform: ${CONFIG.PLATFORM}-${CONFIG.ARCH}`);
    console.log(`Build Type: ${options.debug ? 'debug' : 'release'}`);
    console.log(`Duration: ${minutes}m ${seconds}s`);
    
    const appPath = getApplicationPath();
    if (existsSync(appPath)) {
      const stats = statSync(appPath);
      const sizeMB = Math.round(stats.size / (1024 * 1024));
      console.log(`Application: ${appPath} (${sizeMB}MB)`);
    }
    
    console.log();
    
  } catch (error) {
    console.error('\n[FATAL] Build failed:', error.message);
    if (options.verbose) {
      console.error(error.stack);
    }
    process.exit(1);
  }
}

// Helper functions
function logHeader() {
  console.log('========================================');
  console.log('devManager Desktop Application Builder');
  console.log('========================================');
  console.log(`Build ID: ${CONFIG.BUILD_ID}`);
  console.log(`Platform: ${CONFIG.PLATFORM}-${CONFIG.ARCH}`);
  console.log(`Build Type: ${options.debug ? 'debug' : 'release'}`);
  console.log(`Clean Build: ${options.clean}`);
  console.log(`Parallel Build: ${options.parallel}`);
  console.log(`Version Check: ${options.versionCheck}`);
  console.log(`Started: ${new Date().toISOString()}`);
  console.log();
}

function showHelp() {
  console.log(`
Cross-platform build script for devManager Desktop Application

Usage: node build.js [options]

Options:
  --clean, -c        Clean build directories before building
  --debug, -d        Build in debug mode (default: release)
  --parallel, -p     Enable parallel builds where possible
  --no-version-check Skip version checking
  --verbose, -v       Verbose output
  --help, -h         Show this help message

Examples:
  node build.js                    # Standard release build
  node build.js --clean --debug    # Clean debug build
  node build.js -c -p              # Clean parallel build
`);
}

function generateBuildId() {
  const now = new Date();
  const date = now.toISOString().split('T')[0].replace(/-/g, '');
  const time = now.toTimeString().split(' ')[0].replace(/:/g, '');
  const hash = createHash('md5').update(process.cwd()).digest('hex').substring(0, 6);
  return `${date}_${time}_${hash}`;
}

function execCommand(command, cwd = process.cwd(), opts = {}) {
  const verbose = opts.verbose ?? false;
  if (verbose || options.verbose) {
    console.log(`[EXEC] ${command}`);
  }
  
  try {
    const result = execSync(command, {
      cwd,
      stdio: (verbose || options.verbose) ? 'inherit' : 'pipe',
      encoding: 'utf8',
      timeout: opts.timeout
    });
    return result;
  } catch (error) {
    throw new Error(`Command failed: ${command}\n${error.message}`);
  }
}

async function checkSystemRequirements() {
  console.log('[STEP 1] Checking system requirements...');
  
  // Check disk space
  try {
    const freeSpace = await getDiskSpace();
    if (freeSpace < CONFIG.REQUIRED_DISK_SPACE_GB) {
      console.warn(`[WARNING] Low disk space detected (${freeSpace}GB free). Consider freeing up space.`);
    } else {
      console.log(`[OK] Sufficient disk space available (${freeSpace}GB free)`);
    }
  } catch (error) {
    console.warn('[WARNING] Could not check disk space:', error.message);
  }
}

async function getDiskSpace() {
  return new Promise((resolve, reject) => {
    try {
      if (CONFIG.PLATFORM === 'win32') {
        // wmic was removed in Windows 11/Server 2022, use PowerShell instead
        const output = execCommand(
          'powershell -NoProfile -Command "(Get-PSDrive C).Free"',
          process.cwd(),
          { verbose: false }
        ).trim();
        const freeBytes = parseInt(output, 10);
        if (!isNaN(freeBytes)) {
          const freeSpaceGB = Math.round(freeBytes / (1024 * 1024 * 1024));
          resolve(freeSpaceGB);
        } else {
          reject(new Error('Could not parse disk space'));
        }
      } else {
        const output = execCommand('df -BG .', process.cwd(), { verbose: false });
        const match = output.match(/(\d+)G/);
        if (match) {
          resolve(parseInt(match[1]));
        } else {
          reject(new Error('Could not parse disk space'));
        }
      }
    } catch (error) {
      reject(error);
    }
  });
}

async function checkDependencies() {
  console.log('\n[STEP 2] Checking dependencies...');
  
  const dependencies = [
    { name: 'node', command: 'node --version', minVersion: CONFIG.MIN_NODE_VERSION },
    { name: 'npm', command: 'npm --version', minVersion: null },
    { name: 'go', command: 'go version', minVersion: CONFIG.MIN_GO_VERSION },
    { name: 'wails', command: 'wails version', minVersion: CONFIG.MIN_WAILS_VERSION },
  ];
  
  for (const dep of dependencies) {
    try {
      const rawVersion = execCommand(dep.command, process.cwd(), { verbose: false }).trim();
      console.log(`[OK] Found ${dep.name} ${rawVersion}`);

      // Extract the version number (e.g. "go version go1.21.5" -> "1.21.5",
      // "wails version v2.15.0" -> "2.15.0")
      const semverMatch = rawVersion.match(/(\d+\.\d+\.\d+)/);
      const version = semverMatch ? semverMatch[1] : rawVersion;

      if (options.versionCheck && dep.minVersion) {
        if (!checkVersion(version, dep.minVersion)) {
          throw new Error(`${dep.name} version ${version} is too old. Minimum required: ${dep.minVersion}`);
        }
      }
    } catch (error) {
      if (dep.name === 'wails') {
        console.error(`[ERROR] ${dep.name} is not installed or not in PATH`);
        console.error(`Please install Wails >= ${CONFIG.MIN_WAILS_VERSION} and try again`);
        console.error(`Run: go install github.com/wailsapp/wails/v2/cmd/wails@latest`);
      } else {
        console.error(`[ERROR] ${dep.name} is not installed or not in PATH`);
        console.error(`Please install ${dep.name} and try again`);
      }
      throw error;
    }
  }
}

function checkVersion(current, minimum) {
  const cleanCurrent = current.replace(/^v/, '');
  const cleanMinimum = minimum.replace(/^v/, '');
  
  const currentParts = cleanCurrent.split('.').map(Number);
  const minimumParts = cleanMinimum.split('.').map(Number);
  
  for (let i = 0; i < Math.max(currentParts.length, minimumParts.length); i++) {
    const currentPart = currentParts[i] || 0;
    const minimumPart = minimumParts[i] || 0;
    
    if (currentPart > minimumPart) return true;
    if (currentPart < minimumPart) return false;
  }
  
  return true;
}

async function setupBuildEnvironment() {
  console.log('\n[STEP 3] Setting up build environment...');
  
  // Change to script directory
  process.chdir(__dirname);
  console.log(`[INFO] Working directory: ${process.cwd()}`);
  
  // Clean if requested
  if (options.clean) {
    console.log('\n[STEP 4] Cleaning previous builds...');
    const dirsToClean = ['frontend/dist', 'build', 'bin'];
    
    for (const dir of dirsToClean) {
      if (existsSync(dir)) {
        console.log(`[CLEAN] Removing ${dir}...`);
        rmSync(dir, { recursive: true, force: true });
      }
    }
    
    // Clean Go cache
    console.log('[CLEAN] Cleaning Go build cache...');
    try {
      execCommand('go clean -cache', process.cwd(), { verbose: false });
    } catch (error) {
      console.warn('[WARNING] Could not clean Go cache:', error.message);
    }
    
    console.log('[SUCCESS] Clean completed');
  } else {
    console.log('\n[STEP 4] Skipping clean (use --clean to force clean)');
  }
}

async function buildFrontend() {
  console.log('\n[STEP 5] Building frontend...');
  
  const frontendDir = join(__dirname, 'frontend');
  
  // Install dependencies
  console.log('[EXEC] npm install --no-audit --no-fund');
  execCommand('npm install --no-audit --no-fund', frontendDir);
  
  // Build frontend
  if (options.debug) {
    console.log('[EXEC] npm run build:dev');
    execCommand('npm run build:dev', frontendDir);
  } else {
    console.log('[EXEC] npm run build');
    execCommand('npm run build', frontendDir);
  }
  
  console.log('[SUCCESS] Frontend built successfully');
}

async function buildApplication() {
  console.log('\n[STEP 6] Building Wails application...');
  
  let buildCommand = 'wails build';
  if (!options.debug) {
    buildCommand += ' -upx';
  }
  
  console.log(`[EXEC] ${buildCommand}`);
  execCommand(buildCommand, __dirname);
  
  console.log('[SUCCESS] Wails application built successfully');
}

async function verifyBuild() {
  console.log('\n[STEP 7] Verifying build output...');
  
  const appPath = getApplicationPath();
  
  if (!existsSync(appPath)) {
    throw new Error('Build artifact not found');
  }
  
  const stats = statSync(appPath);
  const sizeMB = Math.round(stats.size / (1024 * 1024));
  console.log(`[OK] Build artifact found: ${appPath} (${sizeMB}MB)`);

  // Note: runtime start verification is intentionally skipped. This is a
  // GUI desktop application and invoking it from the CLI would open a window
  // and block, and GUI apps don't respond to a --version flag. A successful
  // compile + UPX pack already confirms the binary is produced correctly.

  // Generate build report
  await generateBuildReport(sizeMB);
}

function getApplicationPath() {
  const buildDir = join(__dirname, 'build', 'bin');
  
  if (CONFIG.PLATFORM === 'win32') {
    return join(buildDir, 'devmanager.exe');
  } else {
    return join(buildDir, 'devmanager');
  }
}

async function generateBuildReport(sizeMB) {
  const reportPath = join(__dirname, `build-report-${CONFIG.BUILD_ID}.txt`);
  
  const report = `Build Report
=============
Build ID: ${CONFIG.BUILD_ID}
Platform: ${CONFIG.PLATFORM}-${CONFIG.ARCH}
Build Type: ${options.debug ? 'debug' : 'release'}
Start Time: ${new Date().toISOString()}
Duration: ${Math.round((Date.now() - Date.now()) / 1000)}s
Node.js: ${execCommand('node --version', process.cwd(), { silent: true }).trim()}
Go: ${execCommand('go version', process.cwd(), { silent: true }).trim()}
Wails: ${execCommand('wails version', process.cwd(), { silent: true }).trim()}
Artifact Size: ${sizeMB}MB
`;
  
  writeFileSync(reportPath, report);
  console.log(`[INFO] Build report generated: ${reportPath}`);
}

// Run main function
main().catch(console.error);