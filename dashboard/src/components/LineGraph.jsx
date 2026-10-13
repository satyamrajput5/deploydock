// small hand-rolled svg line graph, didn't want to pull in a chart lib just for this
function LineGraph({ data, title, unit = "%" }) {
  const w = 220;
  const h = 80;
  const max = 100; // usage is a percentage

  const points = data
    .map((val, i) => {
      const x = (i / (data.length - 1)) * w;
      const y = h - (val / max) * h;
      return `${x},${y}`;
    })
    .join(" ");

  const latest = data[data.length - 1];

  return (
    <div className="card">
      <div className="graph-box">
        <svg viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" className="graph">
          <line x1="0" y1={h / 2} x2={w} y2={h / 2} className="grid-line" />
          <polyline points={points} className="graph-line" />
        </svg>
        <span className="graph-now">
          {latest}
          {unit}
        </span>
      </div>
      <p className="card-title">{title}</p>
    </div>
  );
}

export default LineGraph;
