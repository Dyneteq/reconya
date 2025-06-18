import React from 'react';
import useEventLogs from '../../hooks/useEventLogs'; // Adjust path as needed
import { EventLog } from '../../models/eventLog.model';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { EventLogIcons } from '../../models/eventLogIcons.model';

const EventLogs = () => {
  const { eventLogs, isLoading, error } = useEventLogs();

  const formatDate = (date: string | Date) => {
    const parsedDate = typeof date === 'string' ? new Date(date) : date;
    return `${parsedDate.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
    })} ${parsedDate.toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    })}`;
  };

  return (
    <div className="mt-5">
      <h6 className="text-success d-block w-100">[ EVENT LOG ]</h6>
      
      {isLoading && eventLogs.length === 0 ? (
        <div className="text-center my-4">
          <div className="spinner-border spinner-border-sm text-success" role="status">
            <span className="visually-hidden">Loading...</span>
          </div>
        </div>
      ) : error ? (
        <div className="alert alert-danger">Error loading event logs: {error.message}</div>
      ) : (
        <table className="table table-dark table-sm table-compact border-dark border-bottom text-success" style={{ fontSize: '13px' }}>
          <tbody>
            {eventLogs.length > 0 ? (
              eventLogs.map((log: EventLog, index: React.Key | null | undefined) => {
                // Handle both snake_case and camelCase properties
                const logType = log.type || log.Type;
                const logDescription = log.description || log.Description;
                const logCreatedAt = log.created_at || log.CreatedAt;
                
                const icon = logType ? EventLogIcons[logType] : null;

                return (
                  <tr key={index}>
                    <td className="bg-transparent text-success px-3">
                      {icon ? (
                        <FontAwesomeIcon icon={icon} className="text-success" />
                      ) : (
                        <span>??</span>
                      )}
                    </td>
                    <td className="bg-transparent text-success px-3">{logDescription}</td>
                    <td className="bg-transparent text-success px-3 text-end">
                      {logCreatedAt ? formatDate(logCreatedAt) : "Unknown"}
                    </td>
                  </tr>
                );
              })
            ) : (
              <tr>
                <td className="bg-transparent text-success px-3" colSpan={3}>No event logs available</td>
              </tr>
            )}
          </tbody>
        </table>
      )}
      
      {/* Small loading indicator when refreshing data */}
      {isLoading && eventLogs.length > 0 && (
        <div className="text-end">
          <small className="text-success">
            <div className="spinner-border spinner-border-sm text-success me-2" role="status">
              <span className="visually-hidden">Loading...</span>
            </div>
            Refreshing...
          </small>
        </div>
      )}
    </div>
  );
};

export default EventLogs;
