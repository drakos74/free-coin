import React from 'react';
import PropTypes from 'prop-types';

const Info = ({coin, duration, fees, trades, value, coins, pnl, threshold, prev,next, deleteCard}) => {
    // const handleDelete = () => {
    //   deleteCard(id);
    // };

    return (
        <div className="col m6 s12">
            <div className="card">
                <div className="card-content">
          <span className="card-title" data-test="stat">
            Value: {coin} - {duration} min ({prev}|{next})
          </span>
                    <div className="card-data">
                        <div data-test="trades">Trades: {trades}</div>
                        <div data-test="trades">Fees: {fees}</div>
                        <div data-test="value">Value: {value}</div>
                        <div data-test="coins">Coins: {coins}</div>
                        <div data-test="pnl">PnL: {pnl}</div>
                        <div data-test="threshold">Threshold: {threshold}</div>
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
