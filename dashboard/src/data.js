// dummy data for now, so the ui can be built before the api is ready.
// TODO: replace with fetch("/deployments") once login + JWT is wired up

export const USER_NAME = "Satyam";

export const budget = {
  monthly: 100, // $ budget
  estimated: 62, // $ est. monthly billing so far
};

export const cpuUsage = [12, 18, 15, 30, 42, 38, 55, 48, 40, 33, 29, 35];
export const ramUsage = [40, 41, 43, 43, 47, 52, 58, 57, 55, 56, 54, 55];

export const deployments = [
  {
    id: 14,
    repo: "github.com/satyamrajput5/olist-dashboard",
    url: "olist-dashboard.deploydock.local",
    status: "running",
    created: "13 Oct, 3:40 PM",
  },
  {
    id: 13,
    repo: "github.com/satyamrajput5/todo-api",
    url: "todo-api.deploydock.local",
    status: "building",
    created: "13 Oct, 2:15 PM",
  },
  {
    id: 12,
    repo: "github.com/satyamrajput5/portfolio",
    url: "portfolio.deploydock.local",
    status: "unhealthy",
    created: "12 Oct, 9:02 PM",
  },
  {
    id: 11,
    repo: "github.com/satyamrajput5/url-shortener",
    url: "-",
    status: "failed",
    created: "11 Oct, 6:30 PM",
  },
  {
    id: 10,
    repo: "github.com/satyamrajput5/chat-app",
    url: "chat-app.deploydock.local",
    status: "rolled_back",
    created: "10 Oct, 11:48 AM",
  },
];
