import BudgetCard from "../components/BudgetCard.jsx";
import LineGraph from "../components/LineGraph.jsx";
import DeploymentsTable from "../components/DeploymentsTable.jsx";
import { cpuUsage, ramUsage, deployments } from "../data.js";

function Dashboard() {
  return (
    <>
      <section className="cards">
        <BudgetCard />
        <LineGraph data={cpuUsage} title="Avg CPU util" />
        <LineGraph data={ramUsage} title="Avg RAM util" />
      </section>

      <h2 className="section-title">Recent Deployments</h2>
      <DeploymentsTable rows={deployments.slice(0, 5)} />
    </>
  );
}

export default Dashboard;
