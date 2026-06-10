import { useEffect, useState } from "react";

import { HealthService } from "../bindings/github.com/ch1lam/autocv/internal/app";

type HealthState = "checking" | "ready" | "error";

function App() {
  const [health, setHealth] = useState<HealthState>("checking");

  useEffect(() => {
    HealthService.Check()
      .then((status) => setHealth(status.status === "ready" ? "ready" : "error"))
      .catch(() => setHealth("error"));
  }, []);

  return (
    <main className="bootstrap-shell">
      <p className="eyebrow">LOCAL-FIRST RESUME WORKBENCH</p>
      <h1>AutoCV</h1>
      <p className="summary">Preparing the local workspace.</p>
      <div className={`health health--${health}`} role="status">
        <span className="health__dot" />
        {health === "checking" && "Checking desktop service"}
        {health === "ready" && "Desktop service ready"}
        {health === "error" && "Desktop service unavailable"}
      </div>
    </main>
  );
}

export default App;
