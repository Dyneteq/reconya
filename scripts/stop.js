#!/usr/bin/env node

const { program } = require('commander');
const Utils = require('./utils');

class ServiceStopper {
  async stop() {
    console.log('==========================================');
    console.log('         Stopping reconYa Backend        ');
    console.log('==========================================\n');

    try {
      let stoppedAny = false;

      // Check for daemon PID file first
      const daemonStopped = await this.stopDaemon();
      if (daemonStopped) stoppedAny = true;

      // Stop backend (port 3008)
      const backendStopped = await Utils.killProcessByPort(3008, 'backend');
      if (backendStopped) stoppedAny = true;

      // Also try to kill any Go processes that might be reconYa
      await this.killreconYaProcesses();

      if (stoppedAny) {
        Utils.log.success('reconYa backend stopped');
      } else {
        Utils.log.info('No reconYa backend was running');
      }

    } catch (error) {
      Utils.log.error('Failed to stop backend: ' + error.message);
      process.exit(1);
    }
  }

  async stopDaemon() {
    const fs = require('fs');
    const path = require('path');
    const pidFile = path.join(process.cwd(), '.reconya.pid');
    
    if (fs.existsSync(pidFile)) {
      try {
        const pid = parseInt(fs.readFileSync(pidFile, 'utf8').trim());
        
        if (pid && !isNaN(pid)) {
          // Check if process exists
          try {
            process.kill(pid, 0); // Check if process exists
            process.kill(pid, 'SIGTERM'); // Kill the process
            Utils.log.info(`Stopped daemon process ${pid}`);
            
            // Remove PID file
            fs.unlinkSync(pidFile);
            return true;
          } catch (error) {
            if (error.code === 'ESRCH') {
              Utils.log.warning('Daemon PID file exists but process not found');
              fs.unlinkSync(pidFile);
            } else {
              throw error;
            }
          }
        }
      } catch (error) {
        Utils.log.warning('Failed to stop daemon: ' + error.message);
        // Clean up PID file anyway
        try {
          fs.unlinkSync(pidFile);
        } catch {}
      }
    }
    
    return false;
  }

  async killreconYaProcesses() {
    try {
      // Kill any processes that might be reconYa backend
      if (Utils.isWindows()) {
        // Windows: Find and kill Go processes running reconYa
        const { stdout } = await Utils.runCommandWithOutput('wmic', ['process', 'where', "CommandLine like '%reconya%' and Name='go.exe'", 'get', 'ProcessId,CommandLine']);
        const lines = stdout.split('\n');
        for (const line of lines) {
          if (line.includes('go.exe') && line.includes('reconya')) {
            const parts = line.trim().split(/\s+/);
            const pid = parts[parts.length - 1];
            if (pid && !isNaN(pid)) {
              await Utils.killProcess(parseInt(pid), true);
              Utils.log.info(`Killed Go process ${pid}`);
            }
          }
        }
      } else {
        // Unix-like: Only kill Go processes running reconya backend (not any process with 'reconya' in command line)
        try {
          // Only kill processes that match 'go run' with reconya/cmd/main.go
          await Utils.runCommand('pkill', ['-f', 'go run .*reconya/cmd/main.go'], { silent: true });
        } catch {
          // Ignore errors - process might not exist
        }
        try {
          // Also try killing compiled reconya binary in backend/cmd
          await Utils.runCommand('pkill', ['-f', 'backend/cmd/reconya'], { silent: true });
        } catch {
          // Ignore errors - process might not exist
        }
      }
    } catch (error) {
      // Ignore errors in this cleanup phase
    }
  }
}

// Main execution
if (require.main === module) {
  program
    .name('reconya-stop')
    .description('Stop reconYa services')
    .version('1.0.0');

  program.parse();

  const stopper = new ServiceStopper();
  stopper.stop().catch(error => {
    Utils.log.error('Failed to stop services: ' + error.message);
    process.exit(1);
  });
}

module.exports = ServiceStopper;