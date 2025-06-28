// src/components/Dashboard/Dashboard.tsx
import React from 'react';
import useAuth from '../../hooks/useAuth';
import useDevices from '../../hooks/useDevices';
import useSystemStatus from '../../hooks/useSystemStatus';
import NetworkMap from '../NetworkMap/NetworkMap';
import Devices from '../Devices/Devices';
import DeviceList from '../DeviceList/DeviceList';
import EventLogs from '../EventLog/EventLogs';
import SystemStatusComponent from '../SystemStatus/SystemStatus';
import TrafficCore from '../TrafficCore/TrafficCore';
import LoadingSpinner from '../Common/LoadingSpinner';
import useNetwork from '../../hooks/useNetwork';
import '../../styles/MinimalAnimations.css';

const Dashboard: React.FC = () => {
  const isAuthenticated = useAuth();

  const { devices, isLoading: devicesLoading, error: devicesError, updateDeviceInState } = useDevices();
  const { systemStatus, isLoading: systemStatusLoading, error: systemStatusError } = useSystemStatus();
  const { network } = useNetwork();

  const localDevice = systemStatus?.local_device || systemStatus?.LocalDevice;

  // Only show loading on initial load, not on subsequent polling
  const isInitialLoading = (devicesLoading && devices.length === 0) || (systemStatusLoading && !systemStatus);
  const error = devicesError || systemStatusError;

  
  if (!isAuthenticated) return null;
  if (isInitialLoading) return <LoadingSpinner message="Loading dashboard data..." />;
  if (error) return (
    <div className="alert alert-danger">
      <h4 className="alert-heading">Error</h4>
      <p>{error.message}</p>
      <hr />
      <p className="mb-0">Please try refreshing the page or contact support if the problem persists.</p>
    </div>
  );

  return (
    <div className="container-fluid">
      <div className="row mt-1">
        <div className="col-md-9">
          {/* Interactive D3.js Network Graph */}
          <NetworkMap devices={devices} localDevice={localDevice} network={network} />
          
          {/* Devices section */}
          <div className="mt-4">
            <Devices devices={devices} localDevice={localDevice} onDeviceUpdate={updateDeviceInState} />
          </div>
          
          {/* Device List right under Devices */}
          <div className="mt-4">
            <DeviceList devices={devices} localDevice={localDevice} />
          </div>
        </div>
        
        <div className="col-md-3 d-flex flex-column">
          <div className="mb-4">
            <SystemStatusComponent systemStatus={systemStatus} network={network} />
          </div>
          <div className="mt-4">
            <TrafficCore devices={devices} localDevice={localDevice} />
          </div>
          <div className="flex-grow-1 mt-4">
            <EventLogs />
          </div>
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
