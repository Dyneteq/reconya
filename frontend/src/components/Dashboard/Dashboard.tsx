import React from 'react';
import useDevices from '../../hooks/useDevices';
import NetworkMap from '../NetworkMap/NetworkMap'; 
import Devices from '../Devices/Devices';
import DeviceList from '../DeviceList/DeviceList';

const Dashboard: React.FC = () => {
  const { devices, isLoading, error } = useDevices();
  console.log('rerun')
  const localDevice = undefined;

  if (isLoading) return <div>Loading...</div>;
  if (error) return <div>Error: {error.message}</div>;

  return (
    <div>
      <NetworkMap devices={devices} localDevice={localDevice} />
      <DeviceList devices={devices} localDevice={localDevice} />
      <Devices devices={devices} />
    </div>
  );
};

export default Dashboard;
