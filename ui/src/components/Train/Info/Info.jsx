import React from 'react';
import PropTypes from 'prop-types';

const Info = ({coin, report}) => {
    // const handleDelete = () => {
    //   deleteCard(id);
    // };

    return (
        <div className="col m6 s12">
            <div className="card">
                <div className="card-content">
          <span className="card-title" data-test="stat">
            Coin: {coin}
          </span>
                    <div className="card-data">
                        <div data-test="trades">Buy / Sell: {report && report.buy} / {report && report.sell}</div>
                        <div data-test="avg">Avg Buy / Sell: {report && report.buy_avg} / {report && report.sell_avg}</div>
                        <div data-test="vol">Vol Buy / Sell: {report && report.buy_vol} / {report && report.sell_vol}</div>
                        <div data-test="fees">Fees: {report && report.fees}</div>
                        <div data-test="wallet">Value: {report && report.wallet}</div>
                        <div data-test="pnl">PnL: {report && report.profit}</div>
                        {/*<span data-test="date">Date: {date}</span>*/}
                    </div>

                    {/*<button className="delete-btn" onClick={handleDelete}>*/}
                    {/*  X*/}
                    {/*</button>*/}
                </div>
            </div>
        </div>
    );
};

Info.propTypes = {
    weight: PropTypes.string,
    height: PropTypes.string,
    id: PropTypes.string,
    date: PropTypes.string,
    bmi: PropTypes.string,
    deleteCard: PropTypes.func
};

export default Info;
