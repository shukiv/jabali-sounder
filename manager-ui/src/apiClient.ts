import axios from "axios";

const client = axios.create({
  baseURL: "/api/v1",
  headers: { "Content-Type": "application/json" },
});

// Error envelope — extract the message from the standard error shape.
// On 401, clear auth and redirect to login.
client.interceptors.response.use(
  (resp) => resp,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem("jabali-sounder-auth");
      localStorage.removeItem("jabali-manager-auth");
      delete client.defaults.headers.common["Authorization"];
      if (window.location.pathname !== "/login" && window.location.pathname !== "/") {
        window.location.reload();
      }
    }
    if (error.response?.data?.error) {
      return Promise.reject(
        new Error(error.response.data.error + (error.response.data.detail ? ": " + error.response.data.detail : "")),
      );
    }
    return Promise.reject(error);
  },
);

export default client;
