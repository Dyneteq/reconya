const chalk = require('chalk');
const { spawn, execSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');
const findProcess = require('find-process');

class Utils {
  static log = {
    info: (msg) => console.log(chalk.blue('[INFO]'), msg),
    success: (msg) => console.log(chalk.green('[SUCCESS]'), msg),
    warning: (msg) => console.log(chalk.yellow('[WARNING]'), msg),
    error: (msg) => console.log(chalk.red('[ERROR]'), msg),
    status: (msg) => console.log(chalk.cyan('[STATUS]'), msg)
  };

  static async sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  static isWindows() {
    return os.platform() === 'win32';
  }

  static isMacOS() {
    return os.platform() === 'darwin';
  }

  static isLinux() {
    return os.platform() === 'linux';
  }

  static getOS() {
    const platform = os.platform();
    if (platform === 'darwin') return 'macos';
    if (platform === 'win32') return 'windows';
    if (platform === 'linux') {
      if (fs.existsSync('/etc/debian_version')) return 'debian';
      if (fs.existsSync('/etc/redhat-release')) return 'redhat';
      if (fs.existsSync('/etc/arch-release')) return 'arch';
      return 'linux';
    }
    return 'unknown';
  }

  static commandExists(command) {
    try {
      if (this.isWindows()) {
        execSync(`where ${command}`, { stdio: 'ignore' });
      } else {
        execSync(`which ${command}`, { stdio: 'ignore' });
      }
      return true;
    } catch {
      return false;
    }
  }

  static async runCommand(command, args = [], options = {}) {
    return new Promise((resolve, reject) => {
      const process = spawn(command, args, {
        stdio: options.silent ? 'ignore' : 'inherit',
        shell: this.isWindows(),
        ...options
      });

      process.on('close', (code) => {
        if (code === 0) {
          resolve(code);
        } else {
          reject(new Error(`Command failed with exit code ${code}`));
        }
      });

      process.on('error', reject);
    });
  }

  static async runCommandWithOutput(command, args = []) {
    return new Promise((resolve, reject) => {
      let stdout = '';
      let stderr = '';

      const process = spawn(command, args, {
        shell: this.isWindows()
      });

      process.stdout.on('data', (data) => {
        stdout += data.toString();
      });

      process.stderr.on('data', (data) => {
        stderr += data.toString();
      });

      process.on('close', (code) => {
        resolve({ code, stdout, stderr });
      });

      process.on('error', reject);
    });
  }

  static async findProcessByPort(port) {
    try {
      if (this.isWindows()) {
        // Use netstat on Windows
        const { stdout } = await this.runCommandWithOutput('netstat', ['-ano']);
        const lines = stdout.split('\n');
        for (const line of lines) {
          if (line.includes(`:${port} `) && line.includes('LISTENING')) {
            const parts = line.trim().split(/\s+/);
            const pid = parts[parts.length - 1];
            return { pid: parseInt(pid) };
          }
        }
      } else {
        // Use lsof on Unix-like systems (macOS, Linux)
        const { stdout } = await this.runCommandWithOutput('lsof', ['-i', `:${port}`, '-t']);
        const pids = stdout.trim().split('\n').filter(pid => pid);
        if (pids.length > 0) {
          return { pid: parseInt(pids[0]) };
        }
      }
      return null;
    } catch {
      return null;
    }
  }

  static async killProcess(pid, force = false) {
    try {
      if (this.isWindows()) {
        const signal = force ? '/F' : '';
        await this.runCommand('taskkill', [signal, '/PID', pid.toString()], { silent: true });
      } else {
        const signal = force ? 'SIGKILL' : 'SIGTERM';
        process.kill(pid, signal);
      }
      return true;
    } catch {
      return false;
    }
  }

  static async killProcessByPort(port, serviceName = 'service') {
    const proc = await this.findProcessByPort(port);
    if (!proc) return false;

    this.log.warning(`Port ${port} is in use by process ${proc.pid} (${serviceName})`);
    this.log.info(`Killing process on port ${port}...`);

    // Try graceful kill first
    let killed = await this.killProcess(proc.pid, false);
    if (killed) {
      await this.sleep(2000);

      // Check if still running
      const stillRunning = await this.findProcessByPort(port);
      if (stillRunning) {
        this.log.warning(`Process ${proc.pid} still running, force killing...`);
        killed = await this.killProcess(proc.pid, true);
        await this.sleep(1000);
      }
    }

    // Verify port is free
    const finalCheck = await this.findProcessByPort(port);
    if (finalCheck) {
      this.log.error(`Failed to free port ${port}. Please manually kill process ${finalCheck.pid}`);
      return false;
    }

    this.log.success(`Port ${port} is now free`);
    return true;
  }

  static validatereconYaDirectory() {
    // Check if we're in the scripts directory, if so, go up one level
    const currentDir = process.cwd();
    const projectRoot = currentDir.endsWith('scripts') ? path.join(currentDir, '..') : currentDir;

    const backendExists = fs.existsSync(path.join(projectRoot, 'backend', 'go.mod'));

    if (!backendExists) {
      this.log.error('Please run this script from the reconYa root directory');
      process.exit(1);
    }

    // Return the project root for use by other functions
    return projectRoot;
  }

  static createEnvFile() {
    const envPath = path.join(process.cwd(), 'backend', '.env');
    const envExamplePath = path.join(process.cwd(), 'backend', '.env.example');

    if (fs.existsSync(envPath)) {
      this.log.success('.env file already exists');
      return;
    }

    if (fs.existsSync(envExamplePath)) {
      this.log.info('Creating .env file from example...');
      fs.copyFileSync(envExamplePath, envPath);
    } else {
      this.log.info('Creating default .env file...');
      const defaultEnv = `LOGIN_USERNAME=admin
LOGIN_PASSWORD=password
NETWORK_RANGE="192.168.1.0/24"
DATABASE_NAME="reconya-dev"
JWT_SECRET_KEY="${this.generateSecretKey()}"
SQLITE_PATH="data/reconya-dev.db"
`;
      fs.writeFileSync(envPath, defaultEnv);
    }
    this.log.success('.env file created');
  }

  static generateSecretKey() {
    const crypto = require('crypto');
    return crypto.randomBytes(32).toString('base64');
  }

  static async waitForPort(port, timeout = 30000) {
    const startTime = Date.now();
    while (Date.now() - startTime < timeout) {
      const process = await this.findProcessByPort(port);
      if (process) return true;
      await this.sleep(1000);
    }
    return false;
  }

  static async setupNmapPermissions() {
    if (this.isWindows()) {
      this.log.info('Windows detected - nmap permissions handled automatically');
      return;
    }

    try {
      const nmapPath = execSync('which nmap', { encoding: 'utf8' }).trim();
      this.log.info('Setting up nmap permissions for MAC address detection...');

      if (this.isMacOS()) {
        await this.runCommand('sudo', ['chown', 'root:admin', nmapPath]);
      } else {
        await this.runCommand('sudo', ['chown', 'root:root', nmapPath]);
      }

      await this.runCommand('sudo', ['chmod', 'u+s', nmapPath]);
      this.log.success('nmap permissions configured');
    } catch (error) {
      this.log.error('Failed to setup nmap permissions: ' + error.message);
      this.log.warning('MAC address detection may not work properly');
    }
  }
}

module.exports = Utils;