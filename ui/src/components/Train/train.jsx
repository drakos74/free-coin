import React, {useEffect, useState} from "react";
import Form from "./Form/Form";
import Bar from "./Bar/Bar";
import Info from "./Info/Info";
import Alert from '@mui/material/Alert';
import AlertTitle from '@mui/material/AlertTitle';
import {Collapse, getTableSortLabelUtilityClass} from "@mui/material";

const Train = () => {
    const [data, setData] = useState({});
    const [alert,setAlert] = useState("");

    const error = (reason) => {
        setAlert(""+reason)
    }

    const handleChange = (data) => {
        console.log(data)

        if (!data.time) {
            return
        }

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
            report:data.report,
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
            <Collapse in={alert.length > 0}>
            <Alert severity="error" onClose={() => {
                setAlert("")
            }}>
                <AlertTitle>Error</AlertTitle>
                <strong>{alert}</strong>
            </Alert>
            </Collapse>
            <div className='row'>
                <div className='col m12 s12'>
                    <Form change={handleChange} error={error}/>
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
                                            report={data.report[detail.coin]}
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