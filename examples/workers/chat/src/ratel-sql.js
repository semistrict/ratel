export function createRatelSQL(fetcher, actor) {
  return {
    exec(sql, ...args) {
      const result = fetcher.fetch("http://ratel-sql/exec", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ actor, sql, args }),
      }).then(async response => {
        if (!response.ok) throw new Error(await response.text());
        return response.json();
      });
      return new RatelSQLCursor(result);
    },
  };
}

class RatelSQLCursor {
  constructor(result) {
    this.result = result;
  }

  async toArray() {
    const result = await this.result;
    return result.rows || [];
  }
}
