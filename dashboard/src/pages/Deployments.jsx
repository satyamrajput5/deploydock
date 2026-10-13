import { useState } from "react";
import DeploymentsTable from "../components/DeploymentsTable.jsx";
import { deployments } from "../data.js";

function Deployments() {
  const [repoUrl, setRepoUrl] = useState("");
  const [rows, setRows] = useState(deployments);

  function handleSubmit(e) {
    e.preventDefault();
    if (!repoUrl.trim()) return;

    // TODO: POST /deployments with the JWT header, for now just fake a row
    const next = {
      id: rows.length ? rows[0].id + 1 : 1,
      repo: repoUrl.replace("https://", ""),
      url: "-",
      status: "pending",
      created: "just now",
    };
    setRows([next, ...rows]);
    setRepoUrl("");
  }

  return (
    <>
      <h2 className="section-title">New deployment</h2>
      <form className="new-form" onSubmit={handleSubmit}>
        <input
          type="text"
          placeholder="https://github.com/user/repo"
          value={repoUrl}
          onChange={(e) => setRepoUrl(e.target.value)}
        />
        <button type="submit">Deploy</button>
      </form>

      <h2 className="section-title">All deployments</h2>
      <DeploymentsTable rows={rows} />
    </>
  );
}

export default Deployments;
