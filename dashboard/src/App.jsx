import { useState } from "react";
import Header from "./components/Header.jsx";
import Sidebar from "./components/Sidebar.jsx";
import Dashboard from "./pages/Dashboard.jsx";
import Deployments from "./pages/Deployments.jsx";
import Budget from "./pages/Budget.jsx";

// no react-router yet, only 3 pages so a simple state is enough
function App() {
  const [page, setPage] = useState("dashboard");
  const [menuOpen, setMenuOpen] = useState(true);

  return (
    <div className="app">
      <Header onMenuClick={() => setMenuOpen(!menuOpen)} />

      <div className="body">
        {menuOpen && <Sidebar current={page} onChange={setPage} />}

        <main className="content">
          {page === "dashboard" && <Dashboard />}
          {page === "deployments" && <Deployments />}
          {page === "budget" && <Budget />}
        </main>
      </div>
    </div>
  );
}

export default App;
