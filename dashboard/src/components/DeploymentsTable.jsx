function DeploymentsTable({ rows }) {
  if (rows.length === 0) {
    return <p className="empty">No deployments yet. Hit "New deployment" to add a GitHub repo.</p>;
  }

  return (
    <table className="table">
      <thead>
        <tr>
          <th>ID</th>
          <th>Repository</th>
          <th>URL</th>
          <th>Status</th>
          <th>Created</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((d) => (
          <tr key={d.id}>
            <td>#{d.id}</td>
            <td>{d.repo}</td>
            <td>{d.url}</td>
            <td>
              <span className={"badge " + d.status}>{d.status.replace("_", " ")}</span>
            </td>
            <td>{d.created}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export default DeploymentsTable;
