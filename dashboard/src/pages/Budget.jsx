import BudgetCard from "../components/BudgetCard.jsx";

// just the card for now, per-deployment cost breakdown can come later
function Budget() {
  return (
    <>
      <h2 className="section-title">Budget</h2>
      <div className="cards single">
        <BudgetCard />
      </div>
    </>
  );
}

export default Budget;
