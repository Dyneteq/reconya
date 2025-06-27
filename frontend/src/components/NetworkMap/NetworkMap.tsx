import React from 'react';
import { Device } from '../../models/device.model';
import { Network } from '../../models/network.model';

interface Props {
  devices: Device[];
  localDevice?: Device;
  network?: Network;
}

// Function to generate all IP addresses in a /24 range
const generateIpRange = (baseIp: string): string[] => {
  const ipParts = baseIp.split('.').map(Number);
  const base = ipParts.slice(0, 3).join('.');
  const ips = [];

  for (let i = 1; i <= 254; i++) {
    ips.push(`${base}.${i}`);
  }

  return ips;
};

// Function to extract base IP from CIDR notation
const getBaseIpFromCIDR = (cidr: string): string => {
  if (!cidr) return '192.168.1.0'; // fallback
  const [ipPart] = cidr.split('/');
  return ipPart;
};

// Helper functions to normalize property access
const getDeviceIPv4 = (device: Device) => device.ipv4 || device.IPv4 || '';
const getDeviceStatus = (device: Device) => device.status || device.Status;

// Function to get the CSS classes for each IP address based on its status
const getDeviceContainerCssClasses = (ip: string, devices: Device[], localDevice?: Device): string => {
  const device = devices.find((device) => getDeviceIPv4(device) === ip);

  if (!device) {
    return 'border border-dark opacity-75 text-muted'; // Style for missing devices
  }

  const deviceStatus = getDeviceStatus(device);
  const deviceIp = getDeviceIPv4(device);
  const localDeviceIp = localDevice ? getDeviceIPv4(localDevice) : '';

  if (deviceStatus === 'online' && deviceIp !== localDeviceIp) {
    return 'border border-success text-success';
  } else if (deviceIp === localDeviceIp) {
    return 'border border-primary text-primary';
  } else if (deviceStatus === 'offline' || deviceStatus === 'unknown') {
    return 'border border-dark text-dark';
  } else if (deviceStatus === 'idle') {
    return 'border border-success text-success opacity-50';
  }

  return 'border border-success'; // Default for existing devices
};

const NetworkMap: React.FC<Props> = ({ devices, localDevice, network }) => {
  // Use network CIDR if available, otherwise fall back to local device IP, then default
  let baseIp: string;
  
  if (network?.cidr || network?.CIDR) {
    // Extract base IP from network CIDR (e.g., "10.0.0.0/24" -> "10.0.0.0")
    const cidr = network.cidr || network.CIDR || '';
    baseIp = getBaseIpFromCIDR(cidr);
  } else if (localDevice) {
    // Fallback to local device IP
    baseIp = getDeviceIPv4(localDevice);
  } else {
    // Default fallback
    baseIp = '192.168.1.0';
  }
  
  const ipRange = generateIpRange(baseIp);

  return (
    <div className="device-container d-flex align-items-start flex-wrap">
      <h6 className="text-success d-block w-100">[ NETWORK MAP ]</h6>
      {ipRange.map((ip, index) => {
        const isExisting = devices.some(device => getDeviceIPv4(device) === ip);
        return (
          <button
            key={index}
            className={`device-mini-box bg-very-dark d-block ps-2 pe-0 m-1 ${getDeviceContainerCssClasses(ip, devices, localDevice)}`}
            title={ip}
            style={{
              borderColor: isExisting ? '#198754' : '#6c757d', // Green border for existing devices
            }}
          >
          </button>
        );
      })}
    </div>
  );
};

export default NetworkMap;