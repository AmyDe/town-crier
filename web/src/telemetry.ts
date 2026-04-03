import { ApplicationInsights } from '@microsoft/applicationinsights-web';

const connectionString = import.meta.env
  .VITE_APPLICATIONINSIGHTS_CONNECTION_STRING;

const appInsights = connectionString
  ? new ApplicationInsights({
      config: {
        connectionString,
        enableAutoRouteTracking: true,
        enableCorsCorrelation: true,
        correlationHeaderDomains: [
          'api-dev.towncrierapp.uk',
          'api.towncrierapp.uk',
        ],
      },
    })
  : null;

appInsights?.loadAppInsights();

export { appInsights };
