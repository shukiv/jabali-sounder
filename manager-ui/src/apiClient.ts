import axios from "axios";

const client = axios.create({
  baseURL: "/api/v1",
  headers: { "Content-Type": "application/json" },
});

// Error envelope — extract the message from the standard error shape.
// On 401, clear auth and redirect to login IF we were authenticated (session
// expired). If there was no session (e.g. a failed login POST), don't reload —
// let the form show the error instead of flashing the page.
client.interceptors.response.use(
  (resp) => resp,
  (error) => {
    if (error.response?.status === 401) {
      const wasAuthed =
        !!localStorage.getItem("jabali-sounder-auth") ||
        !!localStorage.getItem("jabali-manager-auth") ||
        !!client.defaults.headers.common["Authorization"];
      localStorage.removeItem("jabali-sounder-auth");
      localStorage.removeItem("jabali-manager-auth");
      delete client.defaults.headers.common["Authorization"];
      if (wasAuthed) {
        // Full reload to a clean, unauthenticated state -> Login screen.
        window.location.assign("/");
        return new Promise(() => {}); // never resolves; page is navigating away
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
