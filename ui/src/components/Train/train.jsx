import React, {useEffect, useState} from "react";
import Form from "./Form/Form";
import Bar from "./Bar/Bar";
import Info from "./Info/Info";
import Client from "../../api/client"


const Train = () => {
    const [data, setData] = useState({});

    const handleChange = (data) => {
        console.log(data)

        const time = data.time.map((v, _) => {
            return new Date(v)
        })

        const price = data.price.map((p, _) => {
            return {
                x: new Date(p.x),
                y: p.y
            }
        })

        const trades = data.trades.map((t, _) => {
            return {
                x: new Date(t.x),
                y: t.y
            }
        })

        let ml = {}

        Object.keys(data.trigger).forEach(k => {
            ml[k] = {
                buy : data.trigger[k].buy.map((b, _) => {
                    return {
                        x: new Date(b.x),
                        y: b.y
                    }
                }),
                sell : data.trigger[k].sell.map((b, _) => {
                    return {
                        x: new Date(b.x),
                        y: b.y
                    }
                })
            }
        })

        let newData = {
            details: data.details,
            coin: data.details[0].coin,
            time: time,
            trades: trades,
            price: price,
            ml : ml,
        };
        setData(newData);
    };

    // const handleDelete = id => {
    //   storeData('lastState', state);
    //   let newState = state.filter(i => {
    //     return i.id !== id;
    //   });
    //   setState(newState);
    // };

    return (
        <div className='container'>
            <div className='row'>
                <div className='col m12 s12'>
                    <Form change={handleChange}/>
                    <Bar coin={data.coin}
                         labelData={data.time}
                         tradeData={data.trades}
                         priceData={data.price}
                         ml={data.ml}
                    />
                    <div>
                        <div className='row center'>
                            <h4 className='white-text'>7 Day Data</h4>
                        </div>
                        <div className='data-container row'>
                            {data.details && data.details.length > 0 ? (
                                <>
                                    {data.details.map(detail => (
                                        <Info
                                            coin={detail.coin}
                                            duration={detail.duration}
                                            fees={detail.result.fees}
                                            trades={detail.result.trades}
                                            pnl={detail.result.pnl}
                                            value={detail.result.value}
                                            coins={detail.result.coins}
                                            threshold={detail.result.threshold}
                                            prev={detail.prev}
                                            next={detail.next}
                                            // deleteCard={handleDelete}
                                        />
                                    ))}
                                </>
                            ) : (
                                <div className='center white-text'>No log found</div>
                            )}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default Train;