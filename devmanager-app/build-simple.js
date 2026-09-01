#!/usr/bin/env node

/**
 * Simple cross-platform build script for devManager Desktop Application
 * Usage: node build-simple.js [clean] [debug]
 */

import { execSync } from 'child_process';
import { existsSync, rmSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// Parse arguments
const args = process.argv.slice(2);
const clean = args.includes('clean');
const debug = args.includes('debug');

console.log('========================================');
console.log('devManager Desktop Application Builder');
console.log('========================================');
console.log(`Clean: ${clean}`);
console.log(`Debug: ${debug}`);
console.log();

try {
  // Change to script directory
  process.chdir(__dirname);
  console.log(`[INFO] Working directory: ${process.cwd()}`);
  
  // Clean if requested
  if (clean) {
    console.log('[CLEAN] Cleaning previous builds...');
    const dirsToClean = ['frontend/dist', 'build', 'bin'];
    
    for (const dir of dirsToClean) {
      if (existsSync(dir)) {
        console.log(`[CLEAN] Removing ${dir}...`);
        rmSync(dir, { recursive: true, force: true });
      }
    }
    
    // Clean Go cache
    try {
      execSync('go clean -cache', { stdio: 'pipe' });
      console.log('[CLEAN] Go cache cleaned');
    } catch (error) {
      console.warn('[WARNING] Could not clean Go cache');
    }
  }
  
  // Install frontend dependencies
  console.log('[BUILD] Installing frontend dependencies...');
  execSync('npm install --no-audit --no-fund', { 
    cwd: join(__dirname, 'frontend'),
    stdio: 'inherit'
  });
  
  // Build frontend
  console.log('[BUILD] Building frontend...');
  if (debug) {
    execSync('npm run build:dev', { 
      cwd: join(__dirname, 'frontend'),
      stdio: 'inherit'
    });
  } else {
    execSync('npm run build', { 
      cwd: join(__dirname, 'frontend'),
      stdio: 'inherit'
    });
  }
  
  // Build Wails application
  console.log('[BUILD] Building Wails application...');
  if (debug) {
    execSync('wails build', { 
      cwd: __dirname,
      stdio: 'inherit'
    });
  } else {
    execSync('wails build -upx', { 
      cwd: __dirname,
      stdio: 'inherit'
    });
  }
  
  // Verify build
  const appPath = join(__dirname, 'build', 'bin', 'devmanager.exe');
  if (existsSync(appPath)) {
    const stats = require('fs').statSync(appPath);
    const sizeMB = Math.round(stats.size / (1024 * 1024));
    console.log(`[SUCCESS] Build completed!`);
    console.log(`[INFO] Application: ${appPath} (${sizeMB}MB)`);
  } else {
    throw new Error('Build artifact not found');
  }
  
} catch (error) {
  console.error('[ERROR] Build failed:', error.message);
  process.exit(1);
}