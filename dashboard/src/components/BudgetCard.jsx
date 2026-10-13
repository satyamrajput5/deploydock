import { budget } from "../data.js";

function BudgetCard() {
  const { monthly, estimated } = budget;
  const pct = Math.min(estimated / monthly, 1);

  // inner ring is drawn as a dashed circle, dash length = how much of the budget is used
  const r = 38;
  const circumference = 2 * Math.PI * r;

  return (
    <div className="card">
      <div className="budget-row">
        <svg viewBox="0 0 100 100" className="ring">
          <circle cx="50" cy="50" r="46" className="ring-outer" />
          <circle
            cx="50"
            cy="50"
            r={r}
            className="ring-used"
            strokeDasharray={`${circumference * pct} ${circumference}`}
            transform="rotate(-90 50 50)"
          />
          <text x="50" y="50" textAnchor="middle" dominantBaseline="central" className="ring-text">
            $
          </text>
        </svg>

        <ul className="legend">
          <li>
            <span className="dot dot-bill"></span>
            Est. monthly billing <b>${estimated}</b>
          </li>
          <li>
            <span className="dot dot-budget"></span>
            Budget <b>${monthly}</b>
          </li>
        </ul>
      </div>
      <p className="card-title">Budget</p>
    </div>
  );
}

export default BudgetCard;
