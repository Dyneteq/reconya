#!/usr/bin/env node

const { program } = require('commander');
const inquirer = require('inquirer');
const Utils = require('./utils');
const fs = require('fs');
const path = require('path');

class Installer {
  constructor() {
    this.os = Utils.getOS();
  }

  async run() {
    console.log('==========================================');
    console.log('       reconYa Installation Script       ');
    console.log('==========================================\n');

    Utils.log.info(`Detected OS: ${this.os}`);
    Utils.log.info(`Node.js version: ${process.version}`);
    Utils.log.info(`Platform: ${process.platform} ${process.arch}`);
    Utils.log.info(`Working directory: ${process.cwd()}`);

    try {
      Utils.log.info('Step 1: Installing dependencies...');
      await this.installDependencies();

      Utils.log.info('Step 2: Setting up reconYa...');
      await this.setupreconYa();

      Utils.log.info('Step 3: Creating scripts...');
      await this.createScripts();

      console.log('\n==========================================');
      Utils.log.success('reconYa installation completed!');
      console.log('==========================================\n');

      // Try to determine the port and credentials from .env file
      const envPath = path.join(process.cwd(), 'backend', '.env');
      let port = '3008'; // default
      let username = 'admin'; // default
      let password = 'password'; // default

      try {
        if (fs.existsSync(envPath)) {
          const envContent = fs.readFileSync(envPath, 'utf8');

          const portMatch = envContent.match(/^PORT=(.+)$/m);
          if (portMatch) {
            port = portMatch[1].trim();
          }

          const usernameMatch = envContent.match(/^LOGIN_USERNAME=(.+)$/m);
          if (usernameMatch) {
            username = usernameMatch[1].trim().replace(/^["']|["']$/g, '');
          }

          const passwordMatch = envContent.match(/^LOGIN_PASSWORD=(.+)$/m);
          if (passwordMatch) {
            password = passwordMatch[1].trim().replace(/^["']|["']$/g, '');
          }
        }
      } catch (error) {
        // Use default values if can't read .env
      }

      Utils.log.info('To start reconYa, run:');
      console.log('  npm run start\n');
      Utils.log.info('Or use the individual commands:');
      console.log('  npm run status  - Check service status');
      console.log('  npm run start   - Start reconYa');
      console.log('  npm run stop    - Stop reconYa');
      console.log('  npm run install - Reinstall dependencies\n');
      Utils.log.info(`Then open your browser to: http://localhost:${port}`);
      Utils.log.info(`Login credentials: ${username} / ${password}`);

    } catch (error) {
      Utils.log.error('Installation failed: ' + error.message);
      process.exit(1);
    }
  }

  async installDependencies() {
    Utils.log.info('Installing dependencies...');

    switch (this.os) {
      case 'arch':
        await this.installArchDeps();
        break;
      case 'macos':
        await this.installMacOSDeps();
        break;
      case 'debian':
        await this.installDebianDeps();
        break;
      case 'redhat':
        await this.installRedHatDeps();
        break;
      case 'windows':
        await this.installWindowsDeps();
        break;
      default:
        throw new Error(`Unsupported operating system: ${this.os}`);
    }

    await Utils.setupNmapPermissions();
  }

  async installArchDeps() {
    Utils.log.info('Installing dependencies for Arch Linux...');
    // Check if running in container environment
    try {
      const { stdout } = await Utils.runCommandWithOutput('systemd-detect-virt', []);
      if (stdout.trim() !== 'none') {
        Utils.log.warning(`Detected container/VM environment: ${stdout.trim()}`);
        Utils.log.info('This may require additional configuration for nmap functionality');
      }
    } catch (error) {
      // systemd-detect-virt not available, ignore
    }

    // Update package list
    Utils.log.info('Updating package list...');
    try {
      await Utils.runCommand('sudo', ['pacman', '-Syu']);
    } catch (error) {
      throw new Error(`Failed to update package list: ${error.message}`);
    }

    // Install basic tools
    const archPackages = [
      { name: 'curl', cmd: 'curl' },
      { name: 'wget', cmd: 'wget' }
    ];

    for (const pkg of archPackages) {
      if (!Utils.commandExists(pkg.cmd)) {
      Utils.log.info(`Installing ${pkg.name}...`);
      await Utils.runCommand('sudo', ['pacman', '-S', '--noconfirm', pkg.name]);
      } else {
      Utils.log.success(`${pkg.name} is already installed`);
      }
    }

    // Install Go
    if (!Utils.commandExists('go')) {
      Utils.log.info('Installing Go...');
      await Utils.runCommand('sudo', ['pacman', '-S', '--noconfirm', 'go']);
    } else {
      const { stdout } = await Utils.runCommandWithOutput('go', ['version']);
      Utils.log.success(`Go is already installed (${stdout.trim()})`);
    }

    // Install Node.js and npm
    if (!Utils.commandExists('node')) {
      Utils.log.info('Installing Node.js...');
      await Utils.runCommand('sudo', ['pacman', '-S', '--noconfirm', 'nodejs', 'npm']);
    } else {
      const { stdout } = await Utils.runCommandWithOutput('node', ['--version']);
      Utils.log.success(`Node.js is already installed (${stdout.trim()})`);
    }

    // Install nmap
    if (!Utils.commandExists('nmap')) {
      Utils.log.info('Installing nmap...');
      await Utils.runCommand('sudo', ['pacman', '-S', '--noconfirm', 'nmap']);
    } else {
      Utils.log.success('nmap is already installed');
    }
  }


  async installMacOSDeps() {
    Utils.log.info('Installing dependencies for macOS...');

    // Check Homebrew
    if (!Utils.commandExists('brew')) {
      Utils.log.info('Installing Homebrew...');
      await Utils.runCommand('/bin/bash', ['-c',
        '"$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"'
      ]);
    }

    // Install Go
    if (!Utils.commandExists('go')) {
      Utils.log.info('Installing Go...');
      await Utils.runCommand('brew', ['install', 'go']);
    } else {
      const { stdout } = await Utils.runCommandWithOutput('go', ['version']);
      Utils.log.success(`Go is already installed (${stdout.trim()})`);
    }

    // Install Node.js
    if (!Utils.commandExists('node')) {
      Utils.log.info('Installing Node.js...');
      await Utils.runCommand('brew', ['install', 'node']);
    } else {
      const { stdout } = await Utils.runCommandWithOutput('node', ['--version']);
      Utils.log.success(`Node.js is already installed (${stdout.trim()})`);
    }

    // Install nmap
    if (!Utils.commandExists('nmap')) {
      Utils.log.info('Installing nmap...');
      await Utils.runCommand('brew', ['install', 'nmap']);
    } else {
      Utils.log.success('nmap is already installed');
    }
  }

  async installDebianDeps() {
    Utils.log.info('Installing dependencies for Debian-based Linux...');

    // Check if running in container environment
    try {
      const { stdout } = await Utils.runCommandWithOutput('systemd-detect-virt', []);
      if (stdout.trim() !== 'none') {
        Utils.log.warning(`Detected container/VM environment: ${stdout.trim()}`);
        Utils.log.info('This may require additional configuration for nmap functionality');
      }
    } catch (error) {
      // systemd-detect-virt not available, ignore
    }

    // Update package list
    Utils.log.info('Updating package list...');
    try {
      await Utils.runCommand('sudo', ['apt-get', 'update']);
    } catch (error) {
      throw new Error(`Failed to update package list: ${error.message}`);
    }

    // Install basic tools
    await Utils.runCommand('sudo', ['apt-get', 'install', '-y',
      'curl', 'wget', 'software-properties-common', 'apt-transport-https'
    ]);

    // Install Go
    if (!Utils.commandExists('go')) {
      Utils.log.info('Installing Go...');
      const goVersion = '1.21.5';
      // Map Node.js architectures to Go architectures
      let arch;
      switch (process.arch) {
        case 'arm64':
          arch = 'arm64';
          break;
        case 'x64':
        case 'x86_64':
        case 'amd64':
          arch = 'amd64';
          break;
        default:
          throw new Error(`Unsupported architecture: ${process.arch}. Only arm64 and x86_64/amd64 are supported.`);
      }

      const { stdout } = await Utils.runCommandWithOutput('wget', [
        `https://golang.org/dl/go${goVersion}.linux-${arch}.tar.gz`,
        '-O', 'go.tar.gz'
      ]);

      await Utils.runCommand('sudo', ['rm', '-rf', '/usr/local/go']);
      await Utils.runCommand('sudo', ['tar', '-C', '/usr/local', '-xzf', 'go.tar.gz']);
      await Utils.runCommand('rm', ['go.tar.gz']);

      // Add to PATH
      const bashrc = path.join(process.env.HOME, '.bashrc');
      const pathLine = 'export PATH=$PATH:/usr/local/go/bin';

      if (fs.existsSync(bashrc)) {
        const content = fs.readFileSync(bashrc, 'utf8');
        if (!content.includes('/usr/local/go/bin')) {
          fs.appendFileSync(bashrc, `\n${pathLine}\n`);
        }
      }

      process.env.PATH += ':/usr/local/go/bin';
    } else {
      const { stdout } = await Utils.runCommandWithOutput('go', ['version']);
      Utils.log.success(`Go is already installed (${stdout.trim()})`);
    }

    // Install Node.js
    if (!Utils.commandExists('node')) {
      Utils.log.info('Installing Node.js...');
      try {
        // Download the setup script first
        await Utils.runCommand('curl', ['-fsSL',
          'https://deb.nodesource.com/setup_18.x', '-o', '/tmp/nodesource_setup.sh'
        ]);
        // Run the setup script
        await Utils.runCommand('sudo', ['bash', '/tmp/nodesource_setup.sh']);
        // Install Node.js
        await Utils.runCommand('sudo', ['apt-get', 'install', '-y', 'nodejs']);
        // Clean up
        await Utils.runCommand('rm', ['-f', '/tmp/nodesource_setup.sh']);
      } catch (error) {
        throw new Error(`Failed to install Node.js: ${error.message}`);
      }
    } else {
      const { stdout } = await Utils.runCommandWithOutput('node', ['--version']);
      Utils.log.success(`Node.js is already installed (${stdout.trim()})`);
    }

    // Install nmap
    if (!Utils.commandExists('nmap')) {
      Utils.log.info('Installing nmap...');
      await Utils.runCommand('sudo', ['apt-get', 'install', '-y', 'nmap']);
    } else {
      Utils.log.success('nmap is already installed');
    }
  }

  async installRedHatDeps() {
    Utils.log.info('Installing dependencies for Red Hat-based Linux...');

    const pkgMgr = Utils.commandExists('dnf') ? 'dnf' : 'yum';

    // Install basic tools
    await Utils.runCommand('sudo', [pkgMgr, 'install', '-y', 'curl', 'wget']);

    // Install Go (similar to Debian)
    if (!Utils.commandExists('go')) {
      Utils.log.info('Installing Go...');
      const goVersion = '1.21.5';
      // Map Node.js architectures to Go architectures
      let arch;
      switch (process.arch) {
        case 'arm64':
          arch = 'arm64';
          break;
        case 'x64':
        case 'x86_64':
        case 'amd64':
          arch = 'amd64';
          break;
        default:
          throw new Error(`Unsupported architecture: ${process.arch}. Only arm64 and x86_64/amd64 are supported.`);
      }

      await Utils.runCommandWithOutput('wget', [
        `https://golang.org/dl/go${goVersion}.linux-${arch}.tar.gz`,
        '-O', 'go.tar.gz'
      ]);

      await Utils.runCommand('sudo', ['rm', '-rf', '/usr/local/go']);
      await Utils.runCommand('sudo', ['tar', '-C', '/usr/local', '-xzf', 'go.tar.gz']);
      await Utils.runCommand('rm', ['go.tar.gz']);

      // Add to PATH
      const bashrc = path.join(process.env.HOME, '.bashrc');
      const pathLine = 'export PATH=$PATH:/usr/local/go/bin';

      if (fs.existsSync(bashrc)) {
        const content = fs.readFileSync(bashrc, 'utf8');
        if (!content.includes('/usr/local/go/bin')) {
          fs.appendFileSync(bashrc, `\n${pathLine}\n`);
        }
      }

      process.env.PATH += ':/usr/local/go/bin';
    }

    // Install Node.js
    if (!Utils.commandExists('node')) {
      Utils.log.info('Installing Node.js...');
      await Utils.runCommand('curl', ['-fsSL',
        'https://rpm.nodesource.com/setup_18.x', '|', 'sudo', 'bash', '-'
      ]);
      await Utils.runCommand('sudo', [pkgMgr, 'install', '-y', 'nodejs']);
    }

    // Install nmap
    if (!Utils.commandExists('nmap')) {
      Utils.log.info('Installing nmap...');
      await Utils.runCommand('sudo', [pkgMgr, 'install', '-y', 'nmap']);
    }
  }

  async installWindowsDeps() {
    Utils.log.warning('Windows installation requires manual setup:');
    console.log('1. Install Go: https://golang.org/dl/');
    console.log('2. Install Node.js: https://nodejs.org/');
    console.log('3. Install nmap: https://nmap.org/download.html');
    console.log('4. Ensure all tools are in your PATH');

    const { proceed } = await inquirer.prompt([
      {
        type: 'confirm',
        name: 'proceed',
        message: 'Have you installed Go, Node.js, and nmap?',
        default: false
      }
    ]);

    if (!proceed) {
      Utils.log.error('Please install the required dependencies and run the installer again');
      process.exit(1);
    }

    // Verify installations
    if (!Utils.commandExists('go')) throw new Error('Go not found in PATH');
    if (!Utils.commandExists('node')) throw new Error('Node.js not found in PATH');
    if (!Utils.commandExists('nmap')) throw new Error('nmap not found in PATH');
  }

  async setupreconYa() {
    Utils.log.info('Setting up reconYa...');

    // Get project root - since we're running from root now, use current directory
    const projectRoot = process.cwd();
    Utils.log.info(`Project root: ${projectRoot}`);

    // Verify directory structure
    const backendPath = path.join(projectRoot, 'backend');

    if (!fs.existsSync(backendPath)) {
      throw new Error(`Backend directory not found: ${backendPath}`);
    }

    Utils.log.info('Directory structure verified');

    // Create .env file
    const envPath = path.join(projectRoot, 'backend', '.env');
    const envExamplePath = path.join(projectRoot, 'backend', '.env.example');

    if (fs.existsSync(envPath)) {
      Utils.log.success('.env file already exists');
    } else if (fs.existsSync(envExamplePath)) {
      Utils.log.info('Creating .env file from example...');
      fs.copyFileSync(envExamplePath, envPath);
      Utils.log.success('.env file created');
    } else {
      Utils.log.info('Creating default .env file...');
      const defaultEnv = `LOGIN_USERNAME=admin
LOGIN_PASSWORD=password
NETWORK_RANGE="192.168.1.0/24"
DATABASE_NAME="reconya-dev"
JWT_SECRET_KEY="${Utils.generateSecretKey()}"
SQLITE_PATH="data/reconya-dev.db"
`;
      fs.writeFileSync(envPath, defaultEnv);
      Utils.log.success('.env file created');
    }

    // Install Go dependencies
    Utils.log.info('Installing Go dependencies...');

    // Verify Go is available
    if (!Utils.commandExists('go')) {
      throw new Error('Go not found in PATH. Please install Go and ensure it is in your PATH.');
    }

    try {
      Utils.log.info(`Running: go mod download in ${backendPath}`);
      await Utils.runCommand('go', ['mod', 'download'], { cwd: backendPath });
      Utils.log.success('Go dependencies installed');
    } catch (error) {
      throw new Error(`Failed to install Go dependencies: ${error.message}`);
    }

    Utils.log.success('reconYa setup complete!');
  }

  async createScripts() {
    Utils.log.info('Creating management scripts...');

    const projectRoot = process.cwd();
    const packageJsonPath = path.join(projectRoot, 'package.json');

    // Check if package.json already exists
    if (fs.existsSync(packageJsonPath)) {
      Utils.log.success('Package.json already exists');
      return;
    }

    const packageJson = {
      "name": "reconya",
      "version": "1.0.0",
      "description": "Network reconnaissance and asset discovery tool",
      "scripts": {
        "install": "cd scripts && npm install && node install.js",
        "start": "cd scripts && node start.js",
        "stop": "cd scripts && node stop.js",
        "status": "cd scripts && node status.js",
        "uninstall": "cd scripts && node uninstall.js"
      },
      "repository": {
        "type": "git",
        "url": "https://github.com/Dyneteq/reconya-ai-go.git"
      },
      "keywords": [
        "network",
        "reconnaissance",
        "security",
        "nmap",
        "asset-discovery"
      ],
      "author": "reconYa",
      "license": "CC-BY-NC-4.0",
      "engines": {
        "node": ">=14.0.0"
      }
    };

    fs.writeFileSync(packageJsonPath, JSON.stringify(packageJson, null, 2));
    Utils.log.success('Management scripts created');
  }
}

// Main execution
if (require.main === module) {
  program
    .name('reconya-install')
    .description('reconYa installation script')
    .version('1.0.0');

  program.parse();

  const installer = new Installer();
  installer.run().catch(error => {
    Utils.log.error('Installation failed: ' + error.message);
    process.exit(1);
  });
}

module.exports = Installer;