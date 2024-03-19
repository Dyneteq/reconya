import { useState, useEffect } from 'react';
import { fetchDevices } from '../api/axiosConfig';
import { Device } from '../models/device.model';

const useDevices = () => {
  const [devices, setDevices] = useState<Device[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    console.log('effect ran');
    const getDevices = () => {
      try {
        setIsLoading(true);
        fetchDevices().then(data => {
          setDevices(data);
        });
      } catch (error: any) {
        setError(error);
      } finally {
        setIsLoading(false);
      }
    };

    getDevices(); // Fetch devices immediately on component mount

    // Setup polling interval
    const interval = setInterval(getDevices, 3000); // Poll every 3 seconds (adjusted to match comment)

    // Cleanup function to clear interval on component unmount
    return () => clearInterval(interval);
  }, []); // Empty dependency array means this effect runs only on mount and unmount

  return { devices, isLoading, error };
};

export default useDevices;
